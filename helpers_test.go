package bitbuf

import (
	"bytes"
	"encoding/binary"
)

// Helpers shared by the reader, writer and round trip tests. They work one bit
// at a time so they are obviously correct, and are independent of the code under test.

// getBits returns numBits bits of data starting at bit offset, least-significant bit first.
func getBits(data []byte, offset, numBits uint) uint64 {
	var v uint64
	for i := uint(0); i < numBits; i++ {
		pos := offset + i
		v |= uint64(data[pos>>3]>>(pos&7)&1) << i
	}
	return v
}

// setBits sets numBits bits of data starting at bit offset to the low bits of value.
func setBits(data []byte, offset, numBits uint, value uint64) {
	for i := uint(0); i < numBits; i++ {
		pos := offset + i
		if value>>i&1 == 1 {
			data[pos>>3] |= 1 << (pos & 7)
		} else {
			data[pos>>3] &^= 1 << (pos & 7)
		}
	}
}

// signExtend interprets a numBits-wide value as two's complement, using the same
// arithmetic as Source's bf_read::ReadSBitLong: if the sign bit s is set, subtract it twice.
func signExtend(v uint64, numBits uint) int32 {
	if numBits == 0 {
		return 0
	}
	s := int64(1) << (numBits - 1)
	r := int64(v)
	if r >= s {
		r = r - s - s
	}
	return int32(r)
}

// atBitOffset returns payload starting at the given bit offset, in a buffer
// exactly large enough to hold it. Leading and trailing bits are zero.
func atBitOffset(payload []byte, offset uint) []byte {
	numBits := uint(len(payload)) * 8
	out := make([]byte, (offset+numBits+7)/8)
	for i := uint(0); i < numBits; i++ {
		setBits(out, offset+i, 1, getBits(payload, i, 1))
	}
	return out
}

// leBytes returns the little-endian encoding of a fixed-size value.
func leBytes(v interface{}) []byte {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// bufferKinds build a slice with the given contents but different spare capacity.
// Behaviour must not depend on capacity: "exact" panics on any access past len,
// and "padded" leaks garbage into results if bytes past len are read.
var bufferKinds = []struct {
	name string
	make func(data []byte) []byte
}{
	{"exact", func(data []byte) []byte {
		out := make([]byte, len(data))
		copy(out, data)
		return out
	}},
	{"padded", func(data []byte) []byte {
		out := make([]byte, len(data), len(data)+8)
		copy(out, data)
		spare := out[len(data):cap(out)]
		for i := range spare {
			spare[i] = 0xA5
		}
		return out
	}},
}
