package bitbuf

import (
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"testing"
)

type field struct {
	value uint32
	width uint
}

// checkRoundTrip writes fields with a Writer sized exactly to fit, then reads them back.
func checkRoundTrip(t *testing.T, fields []field) {
	t.Helper()

	var totalBits uint
	for _, f := range fields {
		totalBits += f.width
	}

	w := NewWriter(int((totalBits + 7) / 8))
	for i, f := range fields {
		if err := w.WriteUnsignedBitInt32(f.value, f.width); err != nil {
			t.Fatalf("field %d (%d bits): write failed: %v", i, f.width, err)
		}
	}
	if w.BitsWritten() != totalBits {
		t.Fatalf("expected BitsWritten %d, but received %d", totalBits, w.BitsWritten())
	}

	r := NewReader(w.Data())
	for i, f := range fields {
		want := f.value & uint32(uint64(1)<<f.width-1)
		got, err := r.ReadUint32Bits(f.width)
		if err != nil || got != want {
			t.Fatalf("field %d (%d bits): expected %#x, but received %#x (err %v)", i, f.width, want, got, err)
		}
	}
	if left := r.Size() - r.BitsRead(); left >= 8 {
		t.Fatalf("expected only padding bits left, but %d bits remain", left)
	}
}

func TestRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 500; i++ {
		fields := make([]field, 1+rng.Intn(40))
		for j := range fields {
			fields[j] = field{value: rng.Uint32(), width: uint(rng.Intn(33))}
		}
		checkRoundTrip(t, fields)
	}
}

// Every typed write read back with the matching typed read, all at an unaligned offset.
func TestRoundTrip_Typed(t *testing.T) {
	w := NewWriter(64)
	for _, err := range []error{
		w.WriteUnsignedBitInt32(5, 3), // misalign everything that follows
		w.WriteByte(0xC3),
		w.WriteInt8(-57),
		w.WriteUint16(0xBEEF),
		w.WriteInt16(-8375),
		w.WriteUint32(0xDEADBEEF),
		w.WriteInt32(-123456789),
		w.WriteUint64(0xFEDCBA9876543210),
		w.WriteInt64(-5635455352),
		w.WriteUint32(math.Float32bits(2106.3212345)),
		w.WriteUint64(math.Float64bits(-756351.123)),
		w.WriteBytes([]byte{1, 2, 0xFF}),
		w.WriteSignedBitInt32(-3, 5),
		w.WriteString("bitbuf\x00"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	r := NewReader(w.Data())
	reads := []struct {
		name string
		read func() (interface{}, error)
		want interface{}
	}{
		{"ReadUint32Bits", func() (interface{}, error) { return r.ReadUint32Bits(3) }, uint32(5)},
		{"ReadByte", func() (interface{}, error) { return r.ReadByte() }, byte(0xC3)},
		{"ReadInt8", func() (interface{}, error) { return r.ReadInt8() }, int8(-57)},
		{"ReadUint16", func() (interface{}, error) { return r.ReadUint16() }, uint16(0xBEEF)},
		{"ReadInt16", func() (interface{}, error) { return r.ReadInt16() }, int16(-8375)},
		{"ReadUint32", func() (interface{}, error) { return r.ReadUint32() }, uint32(0xDEADBEEF)},
		{"ReadInt32", func() (interface{}, error) { return r.ReadInt32() }, int32(-123456789)},
		{"ReadUint64", func() (interface{}, error) { return r.ReadUint64() }, uint64(0xFEDCBA9876543210)},
		{"ReadInt64", func() (interface{}, error) { return r.ReadInt64() }, int64(-5635455352)},
		{"ReadFloat32", func() (interface{}, error) { return r.ReadFloat32() }, float32(2106.3212345)},
		{"ReadFloat64", func() (interface{}, error) { return r.ReadFloat64() }, float64(-756351.123)},
		{"ReadBytes", func() (interface{}, error) { b, err := r.ReadBytes(3); return string(b), err }, "\x01\x02\xFF"},
		{"ReadInt32Bits", func() (interface{}, error) { return r.ReadInt32Bits(5) }, int32(-3)},
		{"ReadString", func() (interface{}, error) { return r.ReadString(0) }, "bitbuf"},
	}
	for _, tc := range reads {
		got, err := tc.read()
		if err != nil || got != tc.want {
			t.Errorf("%s: expected %v, but received %v (err %v)", tc.name, tc.want, got, err)
		}
	}
}

// Values written with WriteSignedBitInt32 read back unchanged with ReadInt32Bits,
// for every width, including the edges of each width's range.
func TestRoundTrip_SignedBits(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for width := uint(1); width <= 32; width++ {
		lo := -(int64(1) << (width - 1))
		hi := int64(1)<<(width-1) - 1
		var values []int64
		for _, v := range []int64{lo, lo + 1, -1, 0, 1, hi - 1, hi} {
			if v >= lo && v <= hi {
				values = append(values, v)
			}
		}
		for i := 0; i < 20; i++ {
			values = append(values, lo+rng.Int63n(hi-lo+1))
		}

		for offset := uint(0); offset < 8; offset++ {
			w := NewWriter(int((offset + width*uint(len(values)) + 7) / 8))
			w.Seek(offset)
			for _, v := range values {
				if err := w.WriteSignedBitInt32(int32(v), width); err != nil {
					t.Fatalf("width %d, offset %d: writing %d failed: %v", width, offset, v, err)
				}
			}

			r := NewReader(w.Data())
			r.Seek(int(offset))
			for _, v := range values {
				got, err := r.ReadInt32Bits(width)
				if err != nil || int64(got) != v {
					t.Fatalf("width %d, offset %d: expected %d, but received %d (err %v)", width, offset, v, got, err)
				}
			}
		}
	}
}

// FuzzReader checks ReadUint32Bits against getBits for any data, offset and width,
// and that out-of-range reads fail with an error rather than a panic.
func FuzzReader(f *testing.F) {
	f.Add([]byte{0xFF}, uint(0), uint(8))
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, uint(1), uint(32))
	f.Add([]byte{1, 2, 3}, uint(20), uint(5))
	f.Add([]byte{}, uint(0), uint(1))
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8}, uint(8), ^uint(0))

	f.Fuzz(func(t *testing.T, data []byte, offset, width uint) {
		totalBits := uint(len(data)) * 8
		offset %= totalBits + 1

		// ReadBits takes any width, so it must bounds-check without wrapping.
		sut := NewReader(data)
		sut.Seek(int(offset))
		if bits, err := sut.ReadBits(width); width <= totalBits-offset {
			if err != nil || uint(len(bits)) != (width+7)/8 {
				t.Fatalf("ReadBits(%d) at offset %d of %d: expected %d bytes, but received %d (err %v)", width, offset, totalBits, (width+7)/8, len(bits), err)
			}
		} else if err == nil {
			t.Fatalf("ReadBits(%d) at offset %d of %d: expected an error", width, offset, totalBits)
		}

		width %= 40
		sut = NewReader(data)
		sut.Seek(int(offset))
		got, err := sut.ReadUint32Bits(width)

		switch {
		case width > 32:
			if err == nil {
				t.Fatalf("width %d: expected an error", width)
			}
		case offset+width > totalBits:
			if offset == totalBits && width > 0 && err != io.EOF {
				t.Fatalf("width %d at end: expected io.EOF, but received %v", width, err)
			}
			if err == nil {
				t.Fatalf("width %d at offset %d of %d: expected an error", width, offset, totalBits)
			}
		default:
			if want := uint32(getBits(data, offset, width)); err != nil || got != want {
				t.Fatalf("width %d at offset %d: expected %#x, but received %#x (err %v)", width, offset, want, got, err)
			}
			signed := NewReader(data)
			signed.Seek(int(offset))
			gotSigned, err := signed.ReadInt32Bits(width)
			if want := signExtend(getBits(data, offset, width), width); err != nil || gotSigned != want {
				t.Fatalf("signed width %d at offset %d: expected %d, but received %d (err %v)", width, offset, want, gotSigned, err)
			}
			offset += width
		}
		if sut.BitsRead() != offset {
			t.Fatalf("expected BitsRead %d, but received %d", offset, sut.BitsRead())
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
				width: uint(input[0]) % 33,
				value: binary.LittleEndian.Uint32(input[1:5]),
			})
		}
		checkRoundTrip(t, fields)
	})
}
