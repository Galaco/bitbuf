package bitbuf

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"testing"
)

// typedValues is one value of each fixed-size type the Reader reads, with the
// method that reads it. Values have their top bit set to exercise sign handling.
var typedValues = []struct {
	name  string
	value interface{}
	read  func(r *Reader) (interface{}, error)
}{
	{"ReadByte", byte(0xC3), func(r *Reader) (interface{}, error) { return r.ReadByte() }},
	{"ReadUint8", uint8(213), func(r *Reader) (interface{}, error) { return r.ReadUint8() }},
	{"ReadInt8", int8(-57), func(r *Reader) (interface{}, error) { return r.ReadInt8() }},
	{"ReadUint16", uint16(0xBEEF), func(r *Reader) (interface{}, error) { return r.ReadUint16() }},
	{"ReadInt16", int16(-8375), func(r *Reader) (interface{}, error) { return r.ReadInt16() }},
	{"ReadUint32", uint32(0xDEADBEEF), func(r *Reader) (interface{}, error) { return r.ReadUint32() }},
	{"ReadInt32", int32(-123456789), func(r *Reader) (interface{}, error) { return r.ReadInt32() }},
	{"ReadUint64", uint64(0xFEDCBA9876543210), func(r *Reader) (interface{}, error) { return r.ReadUint64() }},
	{"ReadInt64", int64(-5635455352), func(r *Reader) (interface{}, error) { return r.ReadInt64() }},
	{"ReadFloat32", float32(2106.3212345), func(r *Reader) (interface{}, error) { return r.ReadFloat32() }},
	{"ReadFloat64", float64(-756351.123), func(r *Reader) (interface{}, error) { return r.ReadFloat64() }},
}

func TestNewReader(t *testing.T) {
	data := []byte{1, 2, 3}
	sut := NewReader(data)

	if sut.Size() != 24 {
		t.Errorf("Size: expected 24, but received %d", sut.Size())
	}
	if sut.BitsRead() != 0 {
		t.Errorf("BitsRead: expected 0, but received %d", sut.BitsRead())
	}
	if !bytes.Equal(sut.Data(), data) {
		t.Errorf("Data: expected %v, but received %v", data, sut.Data())
	}
}

// Each value is read at every bit offset within a byte, and placed at the very
// end of the buffer so any access past the end is caught.
func TestReader_TypedReads(t *testing.T) {
	for _, tc := range typedValues {
		t.Run(tc.name, func(t *testing.T) {
			payload := leBytes(tc.value)
			for _, kind := range bufferKinds {
				for offset := uint(0); offset < 8; offset++ {
					sut := NewReader(kind.make(atBitOffset(payload, offset)))
					sut.Seek(int(offset))

					got, err := tc.read(sut)
					if err != nil {
						t.Errorf("%s buffer, offset %d: unexpected error: %v", kind.name, offset, err)
						continue
					}
					if got != tc.value {
						t.Errorf("%s buffer, offset %d: expected %v, but received %v", kind.name, offset, tc.value, got)
					}
					if want := offset + uint(len(payload))*8; sut.BitsRead() != want {
						t.Errorf("%s buffer, offset %d: expected BitsRead %d, but received %d", kind.name, offset, want, sut.BitsRead())
					}
				}
			}
		})
	}
}

// Reads every type back to back, from an aligned and an unaligned start.
func TestReader_SequentialReads(t *testing.T) {
	block := []byte{84, 12, 1, 2, 3, 143, 234, 5, 56, 1}

	var payload []byte
	for _, tc := range typedValues {
		payload = append(payload, leBytes(tc.value)...)
	}
	payload = append(payload, block...)
	payload = append(payload, "bitbuf\x00"...)

	for _, kind := range bufferKinds {
		for _, offset := range []uint{0, 3} {
			sut := NewReader(kind.make(atBitOffset(payload, offset)))
			sut.Seek(int(offset))

			for _, tc := range typedValues {
				got, err := tc.read(sut)
				if err != nil || got != tc.value {
					t.Errorf("%s buffer, offset %d, %s: expected %v, but received %v (err %v)", kind.name, offset, tc.name, tc.value, got, err)
				}
			}
			if got, err := sut.ReadBytes(uint(len(block))); err != nil || !bytes.Equal(got, block) {
				t.Errorf("%s buffer, offset %d, ReadBytes: expected %v, but received %v (err %v)", kind.name, offset, block, got, err)
			}
			if got, err := sut.ReadString(0); err != nil || got != "bitbuf" {
				t.Errorf("%s buffer, offset %d, ReadString: expected %q, but received %q (err %v)", kind.name, offset, "bitbuf", got, err)
			}
		}
	}
}

// Compares every width at every offset against getBits, including reads that
// end exactly at the end of the buffer.
func TestReader_ReadUint32Bits(t *testing.T) {
	data := make([]byte, 9)
	rand.New(rand.NewSource(1)).Read(data)
	totalBits := uint(len(data)) * 8

	for _, kind := range bufferKinds {
		t.Run(kind.name, func(t *testing.T) {
			buf := kind.make(data)
			for width := uint(0); width <= 32; width++ {
				for offset := uint(0); offset+width <= totalBits; offset++ {
					sut := NewReader(buf)
					sut.Seek(int(offset))

					got, err := sut.ReadUint32Bits(width)
					want := uint32(getBits(data, offset, width))
					if err != nil || got != want {
						t.Fatalf("width %d at offset %d: expected %#x, but received %#x (err %v)", width, offset, want, got, err)
					}
					if sut.BitsRead() != offset+width {
						t.Fatalf("width %d at offset %d: expected BitsRead %d, but received %d", width, offset, offset+width, sut.BitsRead())
					}
				}
			}
		})
	}
}

func TestReader_ReadInt32Bits(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		width uint
		want  int32
	}{
		{"zero width", []byte{0xFF}, 0, 0},
		{"single bit clear", []byte{0x00}, 1, 0},
		{"single bit set is -1", []byte{0x01}, 1, -1},
		{"nibble max positive", []byte{0x07}, 4, 7},
		{"nibble min negative", []byte{0x08}, 4, -8},
		{"byte negative", []byte{0xC7}, 8, -57},
		{"byte positive", []byte{0x39}, 8, 57},
		{"ignores bits above width", []byte{0xF5}, 4, 5},
		{"full width negative", []byte{0xFF, 0xFF, 0xFF, 0xFF}, 32, -1},
		{"full width min", []byte{0x00, 0x00, 0x00, 0x80}, 32, -2147483648},
		{"full width positive", []byte{0x15, 0xCD, 0x5B, 0x07}, 32, 123456789},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewReader(tc.data).ReadInt32Bits(tc.width)
			if err != nil || got != tc.want {
				t.Errorf("expected %d, but received %d (err %v)", tc.want, got, err)
			}
		})
	}

	// Every width at every offset, against signExtend.
	data := make([]byte, 9)
	rand.New(rand.NewSource(2)).Read(data)
	totalBits := uint(len(data)) * 8
	for _, kind := range bufferKinds {
		buf := kind.make(data)
		for width := uint(0); width <= 32; width++ {
			for offset := uint(0); offset+width <= totalBits; offset++ {
				sut := NewReader(buf)
				sut.Seek(int(offset))

				got, err := sut.ReadInt32Bits(width)
				want := signExtend(getBits(data, offset, width), width)
				if err != nil || got != want {
					t.Fatalf("%s buffer, width %d at offset %d: expected %d, but received %d (err %v)", kind.name, width, offset, want, got, err)
				}
			}
		}
	}
}

func TestReader_BitWidthTooLarge(t *testing.T) {
	for _, width := range []uint{33, 64, 65} {
		sut := NewReader(make([]byte, 16))

		if _, err := sut.ReadUint32Bits(width); err == nil {
			t.Errorf("ReadUint32Bits(%d): expected an error", width)
		}
		if _, err := sut.ReadInt32Bits(width); err == nil {
			t.Errorf("ReadInt32Bits(%d): expected an error", width)
		}
		if sut.BitsRead() != 0 {
			t.Errorf("width %d: a rejected read moved the position to %d", width, sut.BitsRead())
		}
	}
}

func TestReader_ReadBits(t *testing.T) {
	data := []byte{0xEF, 0xBE, 0xAD, 0xDE} // 0xDEADBEEF

	tests := []struct {
		name    string
		offset  int
		numBits uint
		want    []byte
	}{
		{"zero bits", 0, 0, []byte{}},
		{"one bit", 0, 1, []byte{0x01}},
		{"partial trailing byte", 0, 12, []byte{0xEF, 0x0E}},
		{"unaligned byte", 4, 8, []byte{0xEE}},
		{"whole buffer", 0, 32, []byte{0xEF, 0xBE, 0xAD, 0xDE}},
		{"unaligned to end", 1, 31, []byte{0x77, 0xDF, 0x56, 0x6F}},
		{"unaligned to end with partial byte", 4, 28, []byte{0xEE, 0xDB, 0xEA, 0x0D}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, kind := range bufferKinds {
				sut := NewReader(kind.make(data))
				sut.Seek(tc.offset)

				got, err := sut.ReadBits(tc.numBits)
				if err != nil || !bytes.Equal(got, tc.want) {
					t.Errorf("%s buffer: expected %#v, but received %#v (err %v)", kind.name, tc.want, got, err)
				}
			}
		})
	}

	t.Run("past end", func(t *testing.T) {
		sut := NewReader(data)
		sut.Seek(4)

		got, err := sut.ReadBits(29)
		if !errors.Is(err, io.ErrUnexpectedEOF) || got != nil {
			t.Errorf("expected nil and io.ErrUnexpectedEOF, but received %v (err %v)", got, err)
		}
		if sut.BitsRead() != 4 {
			t.Errorf("a failed read moved the position to %d", sut.BitsRead())
		}
	})
}

func TestReader_ReadBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		offset   int
		numBytes uint
		want     []byte
	}{
		{"zero bytes", []byte{1, 2}, 0, 0, []byte{}},
		{"whole buffer", []byte{1, 2, 3, 4, 5}, 0, 5, []byte{1, 2, 3, 4, 5}},
		{"from byte offset to end", []byte{1, 2, 3, 4, 5}, 8, 4, []byte{2, 3, 4, 5}},
		{"unaligned to end", atBitOffset([]byte{1, 2, 0xFF}, 5), 5, 3, []byte{1, 2, 0xFF}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, kind := range bufferKinds {
				sut := NewReader(kind.make(tc.data))
				sut.Seek(tc.offset)

				got, err := sut.ReadBytes(tc.numBytes)
				if err != nil || !bytes.Equal(got, tc.want) {
					t.Errorf("%s buffer: expected %v, but received %v (err %v)", kind.name, tc.want, got, err)
				}
			}
		})
	}
}

func TestReader_ReadOneBit(t *testing.T) {
	sut := NewReader([]byte{0xB2}) // 0b10110010, read least-significant bit first

	for i, want := range []bool{false, true, false, false, true, true, false, true} {
		if got := sut.ReadOneBit(); got != want {
			t.Errorf("bit %d: expected %v, but received %v", i, want, got)
		}
	}

	// Past the end it returns false and does not move.
	if sut.ReadOneBit() {
		t.Error("past end: expected false")
	}
	if sut.BitsRead() != 8 {
		t.Errorf("past end: expected BitsRead 8, but received %d", sut.BitsRead())
	}
}

func TestReader_ReadString(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		offset       int
		maxLength    uint
		want         string
		wantErr      error
		wantBitsRead uint
	}{
		{"stops at and consumes terminator", []byte("hi\x00rest"), 0, 0, "hi", nil, 24},
		{"no terminator reads to end", []byte("abc"), 0, 0, "abc", nil, 24},
		{"maxLength stops early", []byte("hello"), 0, 2, "he", nil, 16},
		{"terminator before maxLength", []byte("hi\x00rest"), 0, 6, "hi", nil, 24},
		{"maxLength past end", []byte("abc"), 0, 10, "abc", io.EOF, 24},
		{"empty buffer", []byte{}, 0, 0, "", nil, 0},
		{"unaligned", atBitOffset([]byte("abc\x00"), 5), 5, 0, "abc", nil, 37},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, kind := range bufferKinds {
				sut := NewReader(kind.make(tc.data))
				sut.Seek(tc.offset)

				got, err := sut.ReadString(tc.maxLength)
				if got != tc.want || err != tc.wantErr {
					t.Errorf("%s buffer: expected %q (err %v), but received %q (err %v)", kind.name, tc.want, tc.wantErr, got, err)
				}
				if sut.BitsRead() != tc.wantBitsRead {
					t.Errorf("%s buffer: expected BitsRead %d, but received %d", kind.name, tc.wantBitsRead, sut.BitsRead())
				}
			}
		})
	}
}

// Reading with no bits left returns io.EOF itself, as io.ByteReader requires.
// Reading with some, but too few, bits left returns an error wrapping
// io.ErrUnexpectedEOF. Neither moves the read position.
func TestReader_EOF(t *testing.T) {
	type read struct {
		name string
		bits uint
		read func(r *Reader) error
	}
	reads := []read{
		{"ReadBytes", 16, func(r *Reader) error { _, err := r.ReadBytes(2); return err }},
		{"ReadBits", 9, func(r *Reader) error { _, err := r.ReadBits(9); return err }},
		{"ReadUint32Bits", 5, func(r *Reader) error { _, err := r.ReadUint32Bits(5); return err }},
		{"ReadInt32Bits", 5, func(r *Reader) error { _, err := r.ReadInt32Bits(5); return err }},
		{"ReadString", 8, func(r *Reader) error { _, err := r.ReadString(1); return err }},
	}
	for _, tc := range typedValues {
		tc := tc
		reads = append(reads, read{tc.name, uint(len(leBytes(tc.value))) * 8, func(r *Reader) error { _, err := tc.read(r); return err }})
	}

	for _, tc := range reads {
		t.Run(tc.name, func(t *testing.T) {
			sut := NewReader(make([]byte, 8))
			sut.Seek(64)
			if err := tc.read(sut); err != io.EOF {
				t.Errorf("no bits left: expected io.EOF, but received %v", err)
			}
			if sut.BitsRead() != 64 {
				t.Errorf("no bits left: a failed read moved the position to %d", sut.BitsRead())
			}

			start := 64 - (tc.bits - 1)
			sut.Seek(int(start))
			if err := tc.read(sut); !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("one bit short: expected io.ErrUnexpectedEOF, but received %v", err)
			}
			if sut.BitsRead() != start {
				t.Errorf("one bit short: a failed read moved the position to %d", sut.BitsRead())
			}
		})
	}
}

// Lengths often come from the data being parsed, so a corrupt negative length
// converted to uint must fail cleanly rather than wrap around the bounds check.
func TestReader_HugeLengths(t *testing.T) {
	reads := []struct {
		name string
		read func(r *Reader) ([]byte, error)
	}{
		{"ReadBits(int32(-1))", func(r *Reader) ([]byte, error) { n := int32(-1); return r.ReadBits(uint(n)) }},
		{"ReadBits(int32(-8))", func(r *Reader) ([]byte, error) { n := int32(-8); return r.ReadBits(uint(n)) }},
		{"ReadBytes(max)", func(r *Reader) ([]byte, error) { return r.ReadBytes(^uint(0)) }},
		{"ReadBytes(overflows bit count)", func(r *Reader) ([]byte, error) { return r.ReadBytes(^uint(0)/8 + 1) }},
	}

	for _, tc := range reads {
		t.Run(tc.name, func(t *testing.T) {
			sut := NewReader(make([]byte, 8))
			sut.Seek(8)

			got, err := tc.read(sut)
			if !errors.Is(err, io.ErrUnexpectedEOF) || got != nil {
				t.Errorf("expected nil and io.ErrUnexpectedEOF, but received %v (err %v)", got, err)
			}
			if sut.BitsRead() != 8 {
				t.Errorf("a failed read moved the position to %d", sut.BitsRead())
			}
		})
	}
}

func TestReader_Seek(t *testing.T) {
	tests := []struct {
		name         string
		offset       int
		wantBitsRead uint
	}{
		{"start", 0, 0},
		{"unaligned", 13, 13},
		{"end", 32, 32},
		{"negative moves to end", -1, 32},
		{"past end moves to end", 33, 32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sut := NewReader(make([]byte, 4))
			sut.Seek(10)
			sut.Seek(tc.offset)

			if sut.BitsRead() != tc.wantBitsRead {
				t.Errorf("expected BitsRead %d, but received %d", tc.wantBitsRead, sut.BitsRead())
			}
		})
	}
}

func TestReader_Reset(t *testing.T) {
	sut := NewReader([]byte{0xEF, 0xBE})

	first, _ := sut.ReadUint16()
	sut.Reset()
	if sut.BitsRead() != 0 {
		t.Errorf("expected BitsRead 0, but received %d", sut.BitsRead())
	}
	if again, err := sut.ReadUint16(); err != nil || again != first {
		t.Errorf("expected to re-read %#x, but received %#x (err %v)", first, again, err)
	}
}
