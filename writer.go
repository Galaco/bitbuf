package bitbuf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unsafe"
)

type Writer struct {
	internalBuffer []byte
	totalBits uint
	currentBit uint
}

func (writer *Writer) Data() []byte {
	return writer.internalBuffer[:len(writer.internalBuffer)-4]
}

func (writer *Writer) BytesWritten() int {
	return int(math.Ceil(float64(writer.currentBit) / 8))
}

func (writer *Writer) WriteByte(val byte) {
	writer.writeSignedBitInt32(int32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteInt8(val int8) {
	writer.writeSignedBitInt32(int32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteUint8(val uint8) {
	writer.writeUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteInt16(val int16) {
	writer.writeSignedBitInt32(int32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteUint16(val uint16) {
	writer.writeUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteInt32(val int32) {
	writer.writeSignedBitInt32(int32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteUint32(val uint32) {
	writer.writeUnsignedBitInt32(uint32(val), uint(unsafe.Sizeof(val)) << 3)
}

func (writer *Writer) WriteString(val string) {
	for _,b := range []byte(val) {
		writer.WriteByte(b)
	}
}

func (writer *Writer) writeUnsignedBitInt32(data uint32, numBits uint) {
	// Force the sign-extension bit to be correct even in the case of overflow.
	//nValue := uint(data)
	//nPreserveBits := (0x7FFFFFFF >> (32 - numBits))
	//nSignExtension := (nValue >> 31) & ^nPreserveBits
	//nValue &= nPreserveBits
	//nValue |= nSignExtension

	writer.writeInternal(uint32(data), numBits, false)
}

func (writer *Writer) writeSignedBitInt32(data int32, numBits uint) {
	// Force the sign-extension bit to be correct even in the case of overflow.
	nValue := int(data)
	nPreserveBits := (0x7FFFFFFF >> (32 - numBits))
	nSignExtension := (nValue >> 31) & ^nPreserveBits
	nValue &= nPreserveBits
	nValue |= nSignExtension

	writer.writeInternal(uint32(nValue), numBits, false)
}

func (writer *Writer) writeInternal(curData uint32, numBits uint, checkRange bool) error {
	if err := writer.ensureInBounds(numBits); err != nil {
		writer.currentBit = writer.totalBits
		return err
	}

	iCurBitMasked := writer.currentBit & 31
	iDWord := uint32(writer.currentBit >> 5)
	writer.currentBit += numBits

	// Mask in a dword.
	//Assert((iDWord * 4 + sizeof(long)) <= (unsigned int)m_nDataBytes)
	pOut := []uint32{
		bytesToUint32(writer.internalBuffer[(iDWord*4):(iDWord*4)+4]),
		bytesToUint32(writer.internalBuffer[(iDWord*4)+4:(iDWord*4)+8]),
	}

	// Rotate data into dword alignment
	curData = (curData << iCurBitMasked) | (curData >> (32 - iCurBitMasked))

	// Calculate bitmasks for first and second word
	temp := uint(1 << (numBits - 1))
	mask1 := uint32((temp * 2 - 1) << iCurBitMasked)
	mask2 := uint32((temp - 1) >> (31 - iCurBitMasked))

	// Only look beyond current word if necessary (avoid access violation)
	i := mask2 & 1
	dword1 := pOut[0]
	dword2 := pOut[i]

	// Drop bits into place
	dword1 ^= (mask1 & (curData ^ dword1))
	dword2 ^= (mask2 & (curData ^ dword2))

	// Note reversed order of writes so that dword1 wins if mask2 == 0 && i == 0
	binary.LittleEndian.PutUint32(writer.internalBuffer[(iDWord*4) + (i*4):(iDWord*4) + (i*4) + 4], dword2)
	binary.LittleEndian.PutUint32(writer.internalBuffer[(iDWord*4):(iDWord*4) + 4], dword1)

	return nil
}

func (writer *Writer) ensureInBounds(numBits uint) error {
	if writer.currentBit + numBits > writer.totalBits {
		return errors.New(fmt.Sprintf("bitbuf attempt oob write by %d bits", (writer.currentBit + numBits) - writer.totalBits))
	}
	return nil
}


func NewWriter(length int) *Writer {
	return & Writer{
		internalBuffer: make([]byte, length + 4),
		totalBits:	    uint(length * 8) + 32,
		currentBit:     0,
	}
}
