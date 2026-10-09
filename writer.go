package bitbuf

import (
	"fmt"
	"math"
	"unsafe"
)

// Writer writes values to a little-endian, least-significant-bit-first bitstream.
type Writer struct {
	internalBuffer []byte
	totalBits      uint
	currentBit     uint
	bitsWritten    uint
}

// Data returns the current written buffer
func (writer *Writer) Data() []byte {
	if writer.BytesWritten() == 0 {
		return make([]byte, 0)
	}
	return writer.internalBuffer[:writer.BytesWritten()]
}

// BitsWritten returns the furthest bit position written to
func (writer *Writer) BitsWritten() uint {
	return writer.bitsWritten
}

// BytesWritten returns number of bytes written
func (writer *Writer) BytesWritten() int {
	return int(math.Ceil(float64(writer.bitsWritten) / 8))
}

// Seek sets the current writer position to the given location.
// Seek index is in bits, NOT bytes!
// Seeking past the end moves to the end, so subsequent writes return an error.
func (writer *Writer) Seek(position uint) {
	if position > writer.totalBits {
		position = writer.totalBits
	}
	writer.currentBit = position
}

// WriteByte writes a single byte
func (writer *Writer) WriteByte(val byte) error {
	return writer.WriteUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteBytes writes a byte slice
func (writer *Writer) WriteBytes(val []byte) error {
	for _, b := range val {
		if err := writer.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

// WriteInt8 writes an Int8
func (writer *Writer) WriteInt8(val int8) error {
	return writer.WriteSignedBitInt32(int32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteUint8 writes a Uint8
func (writer *Writer) WriteUint8(val uint8) error {
	return writer.WriteUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteInt16 writes an Int16
func (writer *Writer) WriteInt16(val int16) error {
	return writer.WriteSignedBitInt32(int32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteUint16 writes a Uint16
func (writer *Writer) WriteUint16(val uint16) error {
	return writer.WriteUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteInt32 writes an Int32
func (writer *Writer) WriteInt32(val int32) error {
	return writer.WriteSignedBitInt32(int32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteUint32 writes a Uint32
func (writer *Writer) WriteUint32(val uint32) error {
	return writer.WriteUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val))<<3)
}

// WriteInt64 writes an Int64
func (writer *Writer) WriteInt64(val int64) error {
	return writer.WriteUint64(uint64(val))
}

// WriteUint64 writes a Uint64
func (writer *Writer) WriteUint64(val uint64) error {
	// Check the whole value fits up front, so a failed write never leaves half a value behind.
	if err := writer.ensureInBounds(uint(unsafe.Sizeof(val)) << 3); err != nil {
		writer.currentBit = writer.totalBits
		return err
	}
	if err := writer.WriteUnsignedBitInt32(uint32(val), 32); err != nil {
		return err
	}
	return writer.WriteUnsignedBitInt32(uint32(val>>32), 32)
}

// WriteString writes a string, byte-by-byte
func (writer *Writer) WriteString(val string) error {
	for _, b := range []byte(val) {
		if err := writer.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

// WriteUnsignedBitInt32 writes a Uint32, but only the specified number of bits (at most 32)
func (writer *Writer) WriteUnsignedBitInt32(data uint32, numBits uint) error {
	return writer.writeInternal(data, numBits)
}

// WriteSignedBitInt32 writes an Int32, but only the specified number of bits (at most 32).
// As in Source's bf_write, an out-of-range value keeps its sign bit: (200, 8) writes 72, not -56.
func (writer *Writer) WriteSignedBitInt32(data int32, numBits uint) error {
	// Force the sign-extension bit to be correct even in the case of overflow.
	nValue := int(data)
	nPreserveBits := 0x7FFFFFFF >> (32 - numBits)
	nSignExtension := (nValue >> 31) & ^nPreserveBits
	nValue &= nPreserveBits
	nValue |= nSignExtension

	return writer.writeInternal(uint32(nValue), numBits)
}

func (writer *Writer) writeInternal(curData uint32, numBits uint) error {
	if numBits > 32 {
		return fmt.Errorf("bitbuf: cannot write %d bits from a 32-bit value", numBits)
	}
	if err := writer.ensureInBounds(numBits); err != nil {
		writer.currentBit = writer.totalBits
		return err
	}
	if numBits == 0 {
		return nil
	}

	// A write of up to 32 bits at any bit offset spans at most 5 bytes.
	// Only touch bytes that are part of the write, so the end of the buffer is safe.
	shift := writer.currentBit & 7
	mask := (uint64(1)<<numBits - 1) << shift
	data := (uint64(curData) << shift) & mask
	firstByte := writer.currentBit >> 3
	lastByte := (writer.currentBit + numBits + 7) >> 3
	for i := firstByte; i < lastByte; i++ {
		writer.internalBuffer[i] = writer.internalBuffer[i]&^byte(mask) | byte(data)
		mask >>= 8
		data >>= 8
	}

	writer.currentBit += numBits
	if writer.currentBit > writer.bitsWritten {
		writer.bitsWritten = writer.currentBit
	}

	return nil
}

// The position never passes the end (see Seek), so the subtraction cannot wrap.
func (writer *Writer) ensureInBounds(numBits uint) error {
	if remaining := writer.totalBits - writer.currentBit; numBits > remaining {
		return fmt.Errorf("bitbuf attempt oob write by %d bits", numBits-remaining)
	}
	return nil
}

// NewWriter returns a new Bitbuf writer
func NewWriter(length int) *Writer {
	return &Writer{
		internalBuffer: make([]byte, length),
		totalBits:      uint(length * 8),
		currentBit:     0,
	}
}
