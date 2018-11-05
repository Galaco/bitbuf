package bitbuf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

type Reader struct {
	internalBuffer bytes.Buffer
	totalBits		 uint
	currentBit       uint
}


// Returns size (in bits, NOT bytes)
func (buf *Reader) Size() uint {
	return buf.totalBits
}


func (buf *Reader) Data() []byte {
	return buf.internalBuffer.Bytes()
}

func (buf *Reader) BitsRead() uint {
	return buf.currentBit
}

func (buf *Reader) Reset() {
	buf.currentBit = 0
}

func (buf *Reader) ReadUint8() (uint8, error) {
	v,err := buf.readInternal(8)
	return uint8(v),err
}

func (buf *Reader) ReadInt8() (int8, error) {
	v,err := buf.readInternal(8)
	return int8(v),err
}

func (buf *Reader) ReadByte() (byte, error) {
	v,err := buf.readInternal(8)
	return byte(v),err
}

func (buf *Reader) ReadInt16() (int16, error) {
	v,err := buf.readInternal(16)
	return int16(v),err
}

func (buf *Reader) ReadUint16() (uint16, error) {
	v,err := buf.readInternal(16)
	return uint16(v),err
}

func (buf *Reader) ReadInt32() (int32, error) {
	v,err := buf.readInternal(32)
	return int32(v),err
}

func (buf *Reader) ReadUint32() (uint32, error) {
	v,err := buf.readInternal(32)
	return uint32(v),err
}

func (buf *Reader) ReadInt64() (int64, error) {
	v,err := buf.ReadBytes(8)
	return bytesToInt64(v),err
}

func (buf *Reader) ReadUint64() (uint64, error) {
	v,err := buf.ReadBytes(8)
	return uint64(bytesToInt64(v)),err
}

func (buf *Reader) ReadFloat32() (float32, error) {
	v,err := buf.ReadBytes(4)
	return bytesToFloat32(v),err
}

func (buf *Reader) ReadFloat64() (float64, error) {
	v,err := buf.ReadBytes(8)
	return bytesToFloat64(v),err
}

func (buf *Reader) ReadBytes(numBytes uint) ([]byte, error) {
	if err := buf.ensureInBounds(numBytes << 3); err != nil {
		return nil,err
	}
	return buf.ReadBits(numBytes << 3)
}

func (buf *Reader) ReadString(maxLength uint) (string, error) {
	// Disregard oob for strings, as we can read until end or null termination
	maxLength = (buf.totalBits - buf.currentBit) / 8

	retVal := make([]byte, 0)
	for i := uint(0); i < maxLength; i++ {
		val, err := buf.ReadByte()
		if val == 0 {
			return string(retVal),err
		}
		retVal = append(retVal, val)
	}
	return string(retVal),nil
}

func (buf *Reader) ReadBits(numBits uint) ([]byte, error) {
	retVal := make([]byte, int(math.Ceil(float64(numBits)/8)))

	//unsigned char *pOut = (unsigned char*)pOutData;
	nBitsLeft := numBits

	// align output to dword boundary
	idx := 0
	//for /* (size_t)pOut & 3) != 0 &&  */nBitsLeft >= 8 {
	//	retVal[idx],_ = buf.ReadByte()
	//	idx++
	//	nBitsLeft -= 8
	//}

	// read dwords
	idx = 0
	for nBitsLeft >= 32 {
		retVal[idx],_ = buf.ReadByte()
		idx++
		retVal[idx],_ = buf.ReadByte()
		idx++
		retVal[idx],_ = buf.ReadByte()
		idx++
		retVal[idx],_ = buf.ReadByte()
		idx++

		nBitsLeft -= 32
	}

	// read remaining bytes
	for nBitsLeft >= 8 {
		retVal[idx],_ = buf.ReadByte()
		idx++

		nBitsLeft -= 8
	}

	// read remaining bits
	if nBitsLeft > 0 {
		v,_ := buf.readInternal(nBitsLeft)
		retVal[idx] = byte(v)
	}

	return retVal, nil
}


func (buf *Reader) readInternal(numBits uint) (uint32,error) {
	if numBits > 64 {
		return 0, errors.New("cannot handle more than 64 bits in a single read")
	}
	err := buf.ensureInBounds(numBits)
	if err != nil {
		return 0, err
	}

	firstByte := buf.currentBit / 8
	startBit := (buf.currentBit & 31) % 8
	//lastBit := buf.currentBit + numBits - 1
	//wordOffset1 := uint(buf.currentBit >> 5)
	//wordOffset2 := uint(lastBit >> 5) + 4
	buf.currentBit += numBits

	bitmask := uint32(2 << (uint(numBits)-1)) - 1

	//dw1 := LoadLittleDWord( (unsigned long* RESTRICT)m_pData, wordOffset1) >> startBit
	//dw2 := LoadLittleDWord( (unsigned long* RESTRICT)m_pData, wordOffset2) << (32 - startBit)
	dw1 := bytesToUint32(buf.internalBuffer.Bytes()[firstByte:firstByte + 4]) >> startBit
	dw2 := uint32(0)
	if buf.totalBits - buf.currentBit > 8 {
		dw2 = bytesToUint32(buf.internalBuffer.Bytes()[firstByte + 4:firstByte + 8]) << (32 - startBit)
	}

	return (dw1 | dw2) & bitmask, nil
}

func (buf *Reader) ensureInBounds(numBits uint) error {
	if buf.currentBit + numBits > buf.totalBits {
		return errors.New(fmt.Sprintf("bitbuf attempt oob read by %d bits", (buf.currentBit + numBits) - buf.totalBits))
	}
	return nil
}

func NewReader(data []byte) *Reader {
	return &Reader{
		internalBuffer: *bytes.NewBuffer(data),
		totalBits:		  uint(len(data) * 8),
		currentBit:       0,
	}
}