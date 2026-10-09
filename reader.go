package bitbuf

import (
	"fmt"
	"io"
)

// Reader reads values from a little-endian, least-significant-bit-first bitstream.
type Reader struct {
	internalBuffer []byte
	totalBits      uint
	currentBit     uint
}

// Size returns size (in bits, NOT bytes)
func (buf *Reader) Size() uint {
	return buf.totalBits
}

// Seek seek to a specific Bit. Not Byte!
// Seeking outside the buffer moves to the end, so subsequent reads return io.EOF.
func (buf *Reader) Seek(offset int) {
	if offset < 0 || uint(offset) > buf.totalBits {
		buf.currentBit = buf.totalBits
		return
	}
	buf.currentBit = uint(offset)
}

// Data returns the entire buffer as []byte
func (buf *Reader) Data() []byte {
	return buf.internalBuffer
}

// BitsRead returns number of bits read
func (buf *Reader) BitsRead() uint {
	return buf.currentBit
}

// Reset seeks back to start (0)
func (buf *Reader) Reset() {
	buf.currentBit = 0
}

// ReadUint8 reads Uint8
func (buf *Reader) ReadUint8() (uint8, error) {
	v, err := buf.readInternal(8)
	if err != nil {
		return 0, err
	}
	return uint8(v), err
}

// ReadInt8 reads Int8
func (buf *Reader) ReadInt8() (int8, error) {
	v, err := buf.readInternal(8)
	if err != nil {
		return 0, err
	}
	return int8(v), err
}

// ReadByte reads Byte
func (buf *Reader) ReadByte() (byte, error) {
	v, err := buf.readInternal(8)
	if err != nil {
		return 0, err
	}
	return byte(v), err
}

// ReadInt16 reads Int16
func (buf *Reader) ReadInt16() (int16, error) {
	v, err := buf.readInternal(16)
	if err != nil {
		return 0, err
	}
	return int16(v), err
}

// ReadUint16 reads Uint16
func (buf *Reader) ReadUint16() (uint16, error) {
	v, err := buf.readInternal(16)
	return uint16(v), err
}

// ReadInt32 reads Int32
func (buf *Reader) ReadInt32() (int32, error) {
	v, err := buf.readInternal(32)
	if err != nil {
		return 0, err
	}
	return int32(v), err
}

// ReadUint32 reads Uint32
func (buf *Reader) ReadUint32() (uint32, error) {
	v, err := buf.readInternal(32)
	if err != nil {
		return 0, err
	}
	return uint32(v), err
}

// ReadInt64 reads Int64
func (buf *Reader) ReadInt64() (int64, error) {
	v, err := buf.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return bytesToInt64(v)
}

// ReadUint64 reads Uint64
func (buf *Reader) ReadUint64() (uint64, error) {
	v, err := buf.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	val, err := bytesToInt64(v)
	return uint64(val), err
}

// ReadFloat32 reads a float32
func (buf *Reader) ReadFloat32() (float32, error) {
	v, err := buf.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return bytesToFloat32(v)
}

// ReadFloat64 reads a float64
func (buf *Reader) ReadFloat64() (float64, error) {
	v, err := buf.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return bytesToFloat64(v)
}

// ReadBytes reads X number of consecutive bytes
func (buf *Reader) ReadBytes(numBytes uint) ([]byte, error) {
	numBits := numBytes << 3
	if numBits>>3 != numBytes {
		// Too large to count in bits, so certainly larger than the buffer.
		numBits = ^uint(0)
	}
	return buf.ReadBits(numBits)
}

// ReadString reads in string data of X length. Underlying implementation same as byte
// Will stop on reaching null terminator.
// If maxLength != 0 will read until null-terminator, EOF OR maxLength read reached.
func (buf *Reader) ReadString(maxLength uint) (string, error) {
	// Disregard oob for strings, as we can read until end or null termination
	if maxLength == 0 {
		maxLength = (buf.totalBits - buf.currentBit) / 8
	}

	retVal := make([]byte, 0)
	for i := uint(0); i < maxLength; i++ {
		val, err := buf.ReadByte()
		if val == 0 {
			return string(retVal), err
		}
		retVal = append(retVal, val)
	}
	return string(retVal), nil
}

// ReadBits reads a specific number of bits.
// Bits are packed into bytes least-significant bit first; a trailing partial byte
// holds the remaining bits in its low bits.
func (buf *Reader) ReadBits(numBits uint) ([]byte, error) {
	if err := buf.ensureInBounds(numBits); err != nil {
		return nil, err
	}

	retVal := make([]byte, (numBits+7)/8)
	for idx := range retVal {
		n := numBits - uint(idx)*8
		if n > 8 {
			n = 8
		}
		// Cannot fail: bounds were checked for the whole read above.
		v, _ := buf.readInternal(n)
		retVal[idx] = byte(v)
	}

	return retVal, nil
}

// ReadUint32Bits reads a specific number of bits (at most 32) that will be treated as a Uint32
func (buf *Reader) ReadUint32Bits(numBits uint) (uint32, error) {
	return buf.readInternal(numBits)
}

// ReadInt32Bits reads a specific number of bits (at most 32) as a two's complement Int32.
// As in Source's bf_read::ReadSBitLong, the top bit read is the sign bit:
// reading 8 bits of 0xC7 returns -57. This reads back values written by WriteSignedBitInt32.
func (buf *Reader) ReadInt32Bits(numBits uint) (int32, error) {
	v, err := buf.readInternal(numBits)
	if err != nil {
		return 0, err
	}
	// Move the sign bit to bit 31, then shift back arithmetically to sign-extend.
	// For numBits 0 both shifts are by 32, which yields 0.
	shift := 32 - numBits
	return int32(v<<shift) >> shift, nil
}

// ReadOneBit reads a single bit as a boolean.
// Reading past the end of the buffer returns false and does not advance.
func (buf *Reader) ReadOneBit() bool {
	if buf.currentBit >= buf.totalBits {
		return false
	}
	value := buf.internalBuffer[buf.currentBit>>3] >> (buf.currentBit & 7)
	buf.currentBit++
	return (value & 1) != 0
}

func (buf *Reader) readInternal(numBits uint) (uint32, error) {
	if numBits > 32 {
		return 0, fmt.Errorf("bitbuf: cannot read %d bits into a 32-bit value", numBits)
	}
	if err := buf.ensureInBounds(numBits); err != nil {
		return 0, err
	}

	// A read of up to 32 bits at any bit offset spans at most 5 bytes.
	// Only touch bytes that are part of the read, so the end of the buffer is safe.
	firstByte := buf.currentBit >> 3
	lastByte := (buf.currentBit + numBits + 7) >> 3
	var word uint64
	for i := firstByte; i < lastByte; i++ {
		word |= uint64(buf.internalBuffer[i]) << ((i - firstByte) << 3)
	}

	word >>= buf.currentBit & 7
	buf.currentBit += numBits

	return uint32(word & (uint64(1)<<numBits - 1)), nil
}

// ensureInBounds returns io.EOF if no bits remain, or an error wrapping
// io.ErrUnexpectedEOF if some, but not enough, bits remain.
// The position never passes the end, so the subtraction cannot wrap, where
// currentBit+numBits could for a huge numBits.
func (buf *Reader) ensureInBounds(numBits uint) error {
	remaining := buf.totalBits - buf.currentBit
	if numBits <= remaining {
		return nil
	}
	if remaining == 0 {
		return io.EOF
	}
	return fmt.Errorf("bitbuf: read of %d bits overruns buffer by %d bits: %w",
		numBits, numBits-remaining, io.ErrUnexpectedEOF)
}

// NewReader returns a new Bitbuf reader.
func NewReader(data []byte) *Reader {
	return &Reader{
		internalBuffer: data,
		totalBits:      uint(len(data) * 8),
		currentBit:     0,
	}
}
