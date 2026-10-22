package bitbuf

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

type field struct {
	value uint32
	width int
}

// checkRoundTrip writes fields, checks the encoding against the bit-by-bit
// reference helpers, then reads the fields back.
func checkRoundTrip(t *testing.T, fields []field) {
	t.Helper()

	var totalBits int
	for _, f := range fields {
		totalBits += f.width
	}

	w := NewLimitedWriter(BitsToBytes(totalBits))
	for _, f := range fields {
		w.WriteBits(f.value, f.width)
	}
	if w.Err() != nil || w.BitsWritten() != totalBits {
		t.Fatalf("wrote %d bits (err %v), expected %d", w.BitsWritten(), w.Err(), totalBits)
	}

	data := w.Bytes()
	offset := uint(0)
	for i, f := range fields {
		want := uint64(f.value) & (uint64(1)<<uint(f.width) - 1)
		if got := getBits(data, offset, uint(f.width)); got != want {
			t.Fatalf("field %d (%d bits): encoded %#x, expected %#x", i, f.width, got, want)
		}
		offset += uint(f.width)
	}

	for _, kind := range bufferKinds {
		r := NewReaderBits(kind.make(data), totalBits)
		for i, f := range fields {
			want := f.value & uint32(uint64(1)<<uint(f.width)-1)
			if got := r.ReadBits(f.width); got != want {
				t.Fatalf("%s: field %d (%d bits): read %#x, expected %#x", kind.name, i, f.width, got, want)
			}
		}
		if r.Err() != nil || r.BitsLeft() != 0 {
			t.Fatalf("%s: err %v, %d bits left", kind.name, r.Err(), r.BitsLeft())
		}
	}
}

func TestRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 500; i++ {
		fields := make([]field, 1+rng.Intn(40))
		for j := range fields {
			fields[j] = field{value: rng.Uint32(), width: rng.Intn(33)}
		}
		checkRoundTrip(t, fields)
	}
}

// Every typed write read back with the matching typed read, at an unaligned
// offset, and checked against the standard library's little-endian encoding.
func TestRoundTrip_Typed(t *testing.T) {
	w := NewWriter(0)
	w.WriteBits(5, 3) // misalign everything that follows
	w.WriteUint8(0xC3)
	w.WriteInt8(-57)
	w.WriteUint16(0xBEEF)
	w.WriteInt16(-8375)
	w.WriteUint32(0xDEADBEEF)
	w.WriteInt32(-123456789)
	w.WriteUint64(0xFEDCBA9876543210)
	w.WriteInt64(-5635455352)
	w.WriteFloat32(2106.3212345)
	w.WriteFloat64(-756351.123)
	w.WriteBytes([]byte{1, 2, 0xFF})
	w.WriteSBits(-3, 5)
	w.WriteString("bitbuf")

	data := w.Bytes()
	payload := make([]byte, 0, len(data))
	for i := uint(0); i+8 <= uint(w.BitsWritten())-3; i += 8 {
		payload = append(payload, byte(getBits(data, 3+i, 8)))
	}
	var want bytes.Buffer
	for _, v := range []interface{}{
		uint8(0xC3), int8(-57), uint16(0xBEEF), int16(-8375), uint32(0xDEADBEEF), int32(-123456789),
		uint64(0xFEDCBA9876543210), int64(-5635455352), float32(2106.3212345), float64(-756351.123),
	} {
		want.Write(leBytes(v))
	}
	if !bytes.HasPrefix(payload, want.Bytes()) {
		t.Fatalf("typed writes differ from little-endian encoding:\n% x\n% x", payload[:want.Len()], want.Bytes())
	}

	r := w.Reader()
	reads := []struct {
		name string
		got  func() interface{}
		want interface{}
	}{
		{"ReadBits", func() interface{} { return r.ReadBits(3) }, uint32(5)},
		{"ReadUint8", func() interface{} { return r.ReadUint8() }, uint8(0xC3)},
		{"ReadInt8", func() interface{} { return r.ReadInt8() }, int8(-57)},
		{"ReadUint16", func() interface{} { return r.ReadUint16() }, uint16(0xBEEF)},
		{"ReadInt16", func() interface{} { return r.ReadInt16() }, int16(-8375)},
		{"ReadUint32", func() interface{} { return r.ReadUint32() }, uint32(0xDEADBEEF)},
		{"ReadInt32", func() interface{} { return r.ReadInt32() }, int32(-123456789)},
		{"ReadUint64", func() interface{} { return r.ReadUint64() }, uint64(0xFEDCBA9876543210)},
		{"ReadInt64", func() interface{} { return r.ReadInt64() }, int64(-5635455352)},
		{"ReadFloat32", func() interface{} { return r.ReadFloat32() }, float32(2106.3212345)},
		{"ReadFloat64", func() interface{} { return r.ReadFloat64() }, float64(-756351.123)},
		{"ReadBytes", func() interface{} { return string(r.ReadBytes(3)) }, "\x01\x02\xFF"},
		{"ReadSBits", func() interface{} { return r.ReadSBits(5) }, int32(-3)},
		{"ReadString", func() interface{} { return r.ReadString() }, "bitbuf"},
	}
	for _, tc := range reads {
		if got := tc.got(); got != tc.want {
			t.Errorf("%s: expected %v, but received %v", tc.name, tc.want, got)
		}
	}
	if r.Err() != nil || r.BitsLeft() != 0 {
		t.Fatalf("err %v, %d bits left", r.Err(), r.BitsLeft())
	}
}

// Values written with WriteSBits read back unchanged with ReadSBits for
// every width and alignment, including the edges of each width's range.
func TestRoundTrip_SignedBits(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for width := 1; width <= 32; width++ {
		lo := -(int64(1) << uint(width-1))
		hi := int64(1)<<uint(width-1) - 1
		values := []int64{lo, lo + 1, -1, 0, 1, hi - 1, hi}
		for i := 0; i < 20; i++ {
			values = append(values, lo+rng.Int63n(hi-lo+1))
		}
		for offset := 0; offset < 8; offset++ {
			w := NewWriter(0)
			w.WriteBits(0, offset)
			for _, v := range values {
				if v >= lo && v <= hi {
					w.WriteSBits(int32(v), width)
				}
			}
			r := w.Reader()
			r.ReadBits(offset)
			for _, v := range values {
				if v < lo || v > hi {
					continue
				}
				if got := r.ReadSBits(width); int64(got) != v {
					t.Fatalf("width %d, offset %d: expected %d, but received %d", width, offset, v, got)
				}
				// The reader's sign extension matches the engine's arithmetic.
				if want := signExtend(uint64(uint32(v))&(uint64(1)<<uint(width)-1), uint(width)); want != int32(v) {
					t.Fatalf("reference sign extension disagrees for %d/%d", v, width)
				}
			}
		}
	}
}

// FuzzReader checks ReadBits and ReadSBits against the reference helpers for
// any data, bit limit, offset and width, and that out-of-range reads set the
// overflow error instead of panicking.
func FuzzReader(f *testing.F) {
	f.Add([]byte{0xFF}, uint(8), uint(0), uint(8))
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, uint(40), uint(1), uint(32))
	f.Add([]byte{1, 2, 3}, uint(21), uint(20), uint(5))
	f.Add([]byte{}, uint(0), uint(0), uint(1))

	f.Fuzz(func(t *testing.T, data []byte, limit, offset, width uint) {
		limit %= uint(len(data))*8 + 1
		offset %= limit + 1
		width %= 33

		for _, kind := range bufferKinds {
			r := NewReaderBits(kind.make(data), int(limit))
			r.Skip(int(offset))
			got := r.ReadBits(int(width))
			if offset+width > limit {
				if got != 0 || !r.Overflowed() || r.BitsLeft() != 0 {
					t.Fatalf("%s: width %d at %d of %d: got %#x overflow=%v left=%d", kind.name, width, offset, limit, got, r.Overflowed(), r.BitsLeft())
				}
				continue
			}
			if want := uint32(getBits(data, offset, width)); got != want || r.Err() != nil {
				t.Fatalf("%s: width %d at %d: expected %#x, but received %#x (err %v)", kind.name, width, offset, want, got, r.Err())
			}
			if r.Pos() != int(offset+width) {
				t.Fatalf("%s: pos %d, expected %d", kind.name, r.Pos(), offset+width)
			}
			s := NewReaderBits(kind.make(data), int(limit))
			s.Skip(int(offset))
			if want := signExtend(getBits(data, offset, width), width); s.ReadSBits(int(width)) != want && width > 0 {
				t.Fatalf("%s: signed width %d at %d: expected %d", kind.name, width, offset, want)
			}
		}
	})
}

// FuzzRoundTrip decodes any input into bit fields, writes them and reads them back.
func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte{8, 0xC3, 0, 0, 0, 32, 0xEF, 0xBE, 0xAD, 0xDE})
	f.Add([]byte{1, 1, 0, 0, 0, 31, 0xFF, 0xFF, 0xFF, 0xFF, 7, 0x55, 0, 0, 0})

	f.Fuzz(func(t *testing.T, input []byte) {
		var fields []field
		for ; len(input) >= 5; input = input[5:] {
			fields = append(fields, field{
				width: int(input[0]) % 33,
				value: binary.LittleEndian.Uint32(input[1:5]),
			})
		}
		checkRoundTrip(t, fields)
	})
}

// FuzzEncodings round-trips the variable-width and float encodings.
func FuzzEncodings(f *testing.F) {
	f.Add(uint32(300), int32(-77), uint64(1<<40), float32(123.25), uint8(3))
	f.Fuzz(func(t *testing.T, u uint32, s int32, u64 uint64, fv float32, offset uint8) {
		if math.IsNaN(float64(fv)) || math.IsInf(float64(fv), 0) {
			fv = 0
		}
		coord := float32(math.Mod(float64(fv), 1<<CoordIntegerBits))
		w := NewWriter(0)
		w.WriteBits(0, int(offset%8))
		w.WriteUBitVar(u)
		w.WriteVarInt32(u)
		w.WriteSignedVarInt32(s)
		w.WriteVarInt64(u64)
		w.WriteSignedVarInt64(int64(u64))
		w.WriteBitCoord(coord)
		r := w.Reader()
		r.ReadBits(int(offset % 8))
		if got := r.ReadUBitVar(); got != u {
			t.Fatalf("UBitVar %d -> %d", u, got)
		}
		if got := r.ReadVarInt32(); got != u {
			t.Fatalf("VarInt32 %d -> %d", u, got)
		}
		if got := r.ReadSignedVarInt32(); got != s {
			t.Fatalf("SignedVarInt32 %d -> %d", s, got)
		}
		if got := r.ReadVarInt64(); got != u64 {
			t.Fatalf("VarInt64 %d -> %d", u64, got)
		}
		if got := r.ReadSignedVarInt64(); got != int64(u64) {
			t.Fatalf("SignedVarInt64 %d -> %d", int64(u64), got)
		}
		got := r.ReadBitCoord()
		if math.Abs(float64(got-coord)) > CoordResolution {
			t.Fatalf("BitCoord %v -> %v", coord, got)
		}
		if r.Err() != nil || r.BitsLeft() != 0 {
			t.Fatalf("err %v, %d bits left", r.Err(), r.BitsLeft())
		}
	})
}
