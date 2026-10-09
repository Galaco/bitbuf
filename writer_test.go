package bitbuf

import (
	"bytes"
	"math/rand"
	"testing"
)

// typedWrites is one value of each type the Writer writes, with the method that writes it.
var typedWrites = []struct {
	name  string
	value interface{}
	write func(w *Writer, v interface{}) error
}{
	{"WriteByte", byte(0xC3), func(w *Writer, v interface{}) error { return w.WriteByte(v.(byte)) }},
	{"WriteUint8", uint8(213), func(w *Writer, v interface{}) error { return w.WriteUint8(v.(uint8)) }},
	{"WriteInt8", int8(-57), func(w *Writer, v interface{}) error { return w.WriteInt8(v.(int8)) }},
	{"WriteUint16", uint16(0xBEEF), func(w *Writer, v interface{}) error { return w.WriteUint16(v.(uint16)) }},
	{"WriteInt16", int16(-8375), func(w *Writer, v interface{}) error { return w.WriteInt16(v.(int16)) }},
	{"WriteUint32", uint32(0xDEADBEEF), func(w *Writer, v interface{}) error { return w.WriteUint32(v.(uint32)) }},
	{"WriteInt32", int32(-123456789), func(w *Writer, v interface{}) error { return w.WriteInt32(v.(int32)) }},
	{"WriteUint64", uint64(0xFEDCBA9876543210), func(w *Writer, v interface{}) error { return w.WriteUint64(v.(uint64)) }},
	{"WriteInt64", int64(-5635455352), func(w *Writer, v interface{}) error { return w.WriteInt64(v.(int64)) }},
	{"WriteBytes", []byte{1, 2, 0xFF}, func(w *Writer, v interface{}) error { return w.WriteBytes(v.([]byte)) }},
	{"WriteString", "bitbuf", func(w *Writer, v interface{}) error { return w.WriteString(v.(string)) }},
}

func encode(v interface{}) []byte {
	if s, ok := v.(string); ok {
		return []byte(s)
	}
	return leBytes(v)
}

func TestNewWriter(t *testing.T) {
	sut := NewWriter(4)

	if sut.BitsWritten() != 0 || sut.BytesWritten() != 0 || len(sut.Data()) != 0 {
		t.Errorf("expected an empty writer, but received %d bits, %d bytes, data %v", sut.BitsWritten(), sut.BytesWritten(), sut.Data())
	}
}

// Each value is written at every bit offset within a byte, into a Writer sized
// exactly to fit, so any access past the end is caught.
func TestWriter_TypedWrites(t *testing.T) {
	for _, tc := range typedWrites {
		t.Run(tc.name, func(t *testing.T) {
			payload := encode(tc.value)
			for offset := uint(0); offset < 8; offset++ {
				want := atBitOffset(payload, offset)
				sut := NewWriter(len(want))
				sut.Seek(offset)

				if err := tc.write(sut, tc.value); err != nil {
					t.Errorf("offset %d: unexpected error: %v", offset, err)
					continue
				}
				if !bytes.Equal(sut.Data(), want) {
					t.Errorf("offset %d: expected %v, but received %v", offset, want, sut.Data())
				}
				if wantBits := offset + uint(len(payload))*8; sut.BitsWritten() != wantBits {
					t.Errorf("offset %d: expected BitsWritten %d, but received %d", offset, wantBits, sut.BitsWritten())
				}
			}
		})
	}
}

func TestWriter_KnownBytes(t *testing.T) {
	sut := NewWriter(9)
	for _, err := range []error{
		sut.WriteInt8(124),
		sut.WriteInt32(212345),
		sut.WriteInt32(-456356),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	want := []byte{124, 121, 61, 3, 0, 92, 9, 249, 255}
	if !bytes.Equal(sut.Data(), want) {
		t.Errorf("expected %v, but received %v", want, sut.Data())
	}
}

// Writes every width at every offset over a random background, and checks that
// exactly the target bits changed.
func TestWriter_WriteUnsignedBitInt32(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	background := make([]byte, 9)
	rng.Read(background)
	totalBits := uint(len(background)) * 8

	for width := uint(0); width <= 32; width++ {
		for offset := uint(0); offset+width <= totalBits; offset++ {
			value := rng.Uint32() // bits above width are set too, and must be ignored
			sut := NewWriter(len(background))
			if err := sut.WriteBytes(background); err != nil {
				t.Fatal(err)
			}
			sut.Seek(offset)

			if err := sut.WriteUnsignedBitInt32(value, width); err != nil {
				t.Fatalf("width %d at offset %d: unexpected error: %v", width, offset, err)
			}
			want := append([]byte(nil), background...)
			setBits(want, offset, width, uint64(value))
			if !bytes.Equal(sut.Data(), want) {
				t.Fatalf("width %d at offset %d: expected %v, but received %v", width, offset, want, sut.Data())
			}
		}
	}
}

func TestWriter_WriteSignedBitInt32(t *testing.T) {
	tests := []struct {
		name  string
		value int32
		width uint
		want  []byte
	}{
		{"single bit", -1, 1, []byte{0x01}},
		{"negative nibble", -2, 4, []byte{0x0E}},
		{"positive nibble", 5, 4, []byte{0x05}},
		{"negative byte", -57, 8, []byte{0xC7}},
		{"full width", -456356, 32, []byte{92, 9, 249, 255}},
		// Out-of-range values keep their sign bit, as in Source's bf_write.
		{"positive overflow stays positive", 200, 8, []byte{72}},
		{"negative overflow stays negative", -200, 8, []byte{0xB8}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sut := NewWriter(4)
			if err := sut.WriteSignedBitInt32(tc.value, tc.width); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(sut.Data(), tc.want) {
				t.Errorf("expected %#v, but received %#v", tc.want, sut.Data())
			}
		})
	}
}

func TestWriter_ZeroWidthWrite(t *testing.T) {
	sut := NewWriter(4)
	sut.WriteByte(0xFF)

	sut.Seek(0)
	if err := sut.WriteUnsignedBitInt32(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sut.Seek(24)
	if err := sut.WriteSignedBitInt32(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(sut.Data(), []byte{0xFF}) || sut.BitsWritten() != 8 {
		t.Errorf("expected [255] and 8 bits, but received %v and %d bits", sut.Data(), sut.BitsWritten())
	}
}

func TestWriter_BitWidthTooLarge(t *testing.T) {
	for _, width := range []uint{33, 64, 65} {
		sut := NewWriter(16)

		if err := sut.WriteUnsignedBitInt32(1, width); err == nil {
			t.Errorf("WriteUnsignedBitInt32(1, %d): expected an error", width)
		}
		if err := sut.WriteSignedBitInt32(1, width); err == nil {
			t.Errorf("WriteSignedBitInt32(1, %d): expected an error", width)
		}
		// A rejected write leaves the position alone.
		sut.WriteByte(7)
		if !bytes.Equal(sut.Data(), []byte{7}) {
			t.Errorf("width %d: expected [7], but received %v", width, sut.Data())
		}
	}
}

// A Writer holds exactly the number of bytes it was created with, whatever the size.
func TestWriter_Overflow(t *testing.T) {
	for length := 0; length <= 9; length++ {
		payload := make([]byte, length)
		for i := range payload {
			payload[i] = byte(i + 1)
		}
		sut := NewWriter(length)

		if err := sut.WriteBytes(payload); err != nil {
			t.Errorf("length %d: unexpected error filling the writer: %v", length, err)
		}
		if err := sut.WriteByte(0xFF); err == nil {
			t.Errorf("length %d: expected an error writing past the end", length)
		}
		if !bytes.Equal(sut.Data(), payload) || sut.BitsWritten() != uint(length)*8 {
			t.Errorf("length %d: expected %v, but received %v (%d bits)", length, payload, sut.Data(), sut.BitsWritten())
		}
	}

	t.Run("unaligned", func(t *testing.T) {
		sut := NewWriter(2)
		sut.WriteUnsignedBitInt32(0xABC, 12)

		if err := sut.WriteByte(0xFF); err == nil {
			t.Error("expected an error writing 8 bits with 4 left")
		}
		// After an overflow the writer is at its end, so even a write that would fit fails.
		if err := sut.WriteUnsignedBitInt32(1, 1); err == nil {
			t.Error("expected an error writing after an overflow")
		}
		if want := []byte{0xBC, 0x0A}; !bytes.Equal(sut.Data(), want) || sut.BitsWritten() != 12 {
			t.Errorf("expected %#v and 12 bits, but received %#v and %d bits", want, sut.Data(), sut.BitsWritten())
		}
	})

	t.Run("multi-byte writes stop at the end", func(t *testing.T) {
		for _, tc := range typedWrites {
			payload := encode(tc.value)
			if len(payload) < 2 {
				continue
			}
			sut := NewWriter(1)
			if err := tc.write(sut, tc.value); err == nil {
				t.Errorf("%s: expected an error writing %d bytes into 1", tc.name, len(payload))
			}
		}
	})

	t.Run("64 bit write never writes half a value", func(t *testing.T) {
		sut := NewWriter(7)
		if err := sut.WriteUint64(0xFFFFFFFFFFFFFFFF); err == nil {
			t.Error("expected an error writing 8 bytes into 7")
		}
		if len(sut.Data()) != 0 || sut.BitsWritten() != 0 {
			t.Errorf("expected nothing written, but received %v (%d bits)", sut.Data(), sut.BitsWritten())
		}
	})
}

func TestWriter_Seek(t *testing.T) {
	tests := []struct {
		name            string
		run             func(w *Writer) error
		want            []byte
		wantBitsWritten uint
		wantErr         bool
	}{
		{"overwrite inside written data", func(w *Writer) error {
			w.WriteUint16(0xAAAA)
			w.Seek(4)
			return w.WriteUnsignedBitInt32(0x5, 4)
		}, []byte{0x5A, 0xAA}, 16, false},
		{"seek back and write past the end", func(w *Writer) error {
			w.WriteUint16(0xAAAA)
			w.Seek(8)
			return w.WriteUint16(0xBBBB)
		}, []byte{0xAA, 0xBB, 0xBB}, 24, false},
		{"seek forward leaves a zero gap", func(w *Writer) error {
			w.WriteByte(1)
			w.Seek(16)
			return w.WriteByte(3)
		}, []byte{1, 0, 3}, 24, false},
		{"seek past the end moves to the end", func(w *Writer) error {
			w.Seek(1000)
			return w.WriteUnsignedBitInt32(1, 1)
		}, []byte{}, 0, true},
		{"seek to max uint", func(w *Writer) error {
			w.Seek(^uint(0))
			return w.WriteByte(1)
		}, []byte{}, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sut := NewWriter(4)
			err := tc.run(sut)

			if (err != nil) != tc.wantErr {
				t.Errorf("expected error %v, but received %v", tc.wantErr, err)
			}
			if !bytes.Equal(sut.Data(), tc.want) || sut.BitsWritten() != tc.wantBitsWritten {
				t.Errorf("expected %#v (%d bits), but received %#v (%d bits)", tc.want, tc.wantBitsWritten, sut.Data(), sut.BitsWritten())
			}
		})
	}
}
