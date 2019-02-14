package bitbuf

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

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

// BitsWritten returns number of bits written
func (writer *Writer) BitsWritten() uint {
	return writer.bitsWritten
}

// BytesWritten returns number of bytes written
func (writer *Writer) BytesWritten() int {
	return int(math.Ceil(float64(writer.bitsWritten) / 8))
}

// Seek sets the current writer position to the given location.
// Seek index is in bits, NOT bytes!
func (writer *Writer) Seek(position uint) {
	if writer.totalBits > position {
		writer.currentBit = writer.totalBits
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
	raw := make([]byte, 8)
	binary.LittleEndian.PutUint64(raw, uint64(val))
	err := writer.WriteUnsignedBitInt32(uint32(binary.LittleEndian.Uint32(raw[:4])), uint(unsafe.Sizeof(val))<<3)
	if err != nil {
		return err
	}
	return writer.WriteUnsignedBitInt32(uint32(binary.LittleEndian.Uint32(raw[4:8])), uint(unsafe.Sizeof(val))<<3)
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

// WriteUnsignedBitInt32 writes a Uint32, but only the specified number of bits
func (writer *Writer) WriteUnsignedBitInt32(data uint32, numBits uint) error {
	// Force the sign-extension bit to be correct even in the case of overflow.
	//nValue := uint(data)
	//nPreserveBits := (0x7FFFFFFF >> (32 - numBits))
	//nSignExtension := (nValue >> 31) & ^nPreserveBits
	//nValue &= nPreserveBits
	//nValue |= nSignExtension

	return writer.writeInternal(uint32(data), numBits, false)
}

// WriteSignedBitInt32 writes an Int32, but only the specified number of bits
func (writer *Writer) WriteSignedBitInt32(data int32, numBits uint) error {
	// Force the sign-extension bit to be correct even in the case of overflow.
	nValue := int(data)
	nPreserveBits := 0x7FFFFFFF >> (32 - numBits)
	nSignExtension := (nValue >> 31) & ^nPreserveBits
	nValue &= nPreserveBits
	nValue |= nSignExtension

	return writer.writeInternal(uint32(nValue), numBits, false)
}

func (writer *Writer) writeInternal(curData uint32, numBits uint, checkRange bool) error {
	if err := writer.ensureInBounds(numBits); err != nil {
		writer.currentBit = writer.totalBits
		return err
	}

	iCurBitMasked := writer.currentBit & 31
	iDWord := uint32(writer.currentBit >> 5)
	if writer.currentBit == writer.bitsWritten {
		writer.bitsWritten += numBits
	}
	writer.currentBit += numBits

	// Mask in a dword.
	//Assert((iDWord * 4 + sizeof(long)) <= (unsigned int)m_nDataBytes)
	pOut := make([]uint32, 2)
	pOut[0], _ = bytesToUint32(writer.internalBuffer[(iDWord * 4) : (iDWord*4)+4])
	pOut[1], _ = bytesToUint32(writer.internalBuffer[(iDWord*4)+4 : (iDWord*4)+8])

	// Rotate data into dword alignment
	curData = (curData << iCurBitMasked) | (curData >> (32 - iCurBitMasked))

	// Calculate bitmasks for first and second word
	temp := uint(1 << (numBits - 1))
	mask1 := uint32((temp*2 - 1) << iCurBitMasked)
	mask2 := uint32((temp - 1) >> (31 - iCurBitMasked))

	// Only look beyond current word if necessary (avoid access violation)
	i := mask2 & 1
	dword1 := pOut[0]
	dword2 := pOut[i]

	// Drop bits into place
	dword1 ^= mask1 & (curData ^ dword1)
	dword2 ^= mask2 & (curData ^ dword2)

	// Note reversed order of writes so that dword1 wins if mask2 == 0 && i == 0
	binary.LittleEndian.PutUint32(writer.internalBuffer[(iDWord*4)+(i*4):(iDWord*4)+(i*4)+4], dword2)
	binary.LittleEndian.PutUint32(writer.internalBuffer[(iDWord*4):(iDWord*4)+4], dword1)

	return nil
}

func (writer *Writer) ensureInBounds(numBits uint) error {
	if writer.currentBit+numBits > writer.totalBits {
		return fmt.Errorf("bitbuf attempt oob write by %d bits", (writer.currentBit+numBits)-writer.totalBits)
	}
	return nil
}

// NewWriter returns a new Bitbuf writer
func NewWriter(length int) *Writer {
	return &Writer{
		internalBuffer: make([]byte, length+4),
		totalBits:      uint(length*8) + 32,
		currentBit:     0,
	}
}
