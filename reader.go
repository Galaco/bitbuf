package bitbuf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

type Reader struct {
	internalBuffer bytes.Buffer
	totalBits      uint
	currentBit     uint
}

// Size returns size (in bits, NOT bytes)
func (buf *Reader) Size() uint {
	return buf.totalBits
}

// Seek seek to a specific Bit. Not Byte!
func (buf *Reader) Seek(offset int) {
	buf.currentBit = uint(offset)
}

// Data returns the entire buffer as []byte
func (buf *Reader) Data() []byte {
	return buf.internalBuffer.Bytes()
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
	if err := buf.ensureInBounds(numBytes << 3); err != nil {
		return nil, err
	}
	return buf.ReadBits(numBytes << 3)
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
func (buf *Reader) ReadBits(numBits uint) ([]byte, error) {
	retVal := make([]byte, int(math.Ceil(float64(numBits)/8)))

	//unsigned char *pOut = (unsigned char*)pOutData;
	nBitsLeft := numBits

	// align output to dword boundary
	idx := 0

	// read dwords
	idx = 0
	for nBitsLeft >= 32 {
		retVal[idx], _ = buf.ReadByte()
		idx++
		retVal[idx], _ = buf.ReadByte()
		idx++
		retVal[idx], _ = buf.ReadByte()
		idx++
		retVal[idx], _ = buf.ReadByte()
		idx++

		nBitsLeft -= 32
	}

	// read remaining bytes
	for nBitsLeft >= 8 {
		retVal[idx], _ = buf.ReadByte()
		idx++

		nBitsLeft -= 8
	}

	// read remaining bits
	if nBitsLeft > 0 {
		v, _ := buf.readInternal(nBitsLeft)
		retVal[idx] = byte(v)
	}

	return retVal, nil
}

// ReadUint32Bits reads a specific number of bits that will be treated as a Uint32
func (buf *Reader) ReadUint32Bits(numBits uint) (uint32, error) {
	return buf.readInternal(numBits)
}

// ReadInt32Bits reads a specific number of bits that will be treated as an Int32
func (buf *Reader) ReadInt32Bits(numBits uint) (int32, error) {
	v, err := buf.readInternal(numBits)
	return int32(v), err
}

// ReadOneBit reads a single bit as a boolean
func (buf *Reader) ReadOneBit() bool {
	value := uint8(buf.internalBuffer.Bytes()[buf.currentBit>>3] >> (buf.currentBit & 7))
	buf.currentBit++
	return (value & 1) != 0
}

func (buf *Reader) readInternal(numBits uint) (uint32, error) {
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

	bitmask := uint32(2<<(uint(numBits)-1)) - 1

	//dw1 := LoadLittleDWord( (unsigned long* RESTRICT)m_pData, wordOffset1) >> startBit
	//dw2 := LoadLittleDWord( (unsigned long* RESTRICT)m_pData, wordOffset2) << (32 - startBit)
	dw1, _ := bytesToUint32(buf.internalBuffer.Bytes()[firstByte : firstByte+4])
	dw1 = dw1 >> startBit
	dw2 := uint32(0)
	if buf.totalBits-buf.currentBit >= 64 {
		dw2, _ = bytesToUint32(buf.internalBuffer.Bytes()[firstByte+4 : firstByte+8])
		dw2 = dw2 << (32 - startBit)
	}

	return (dw1 | dw2) & bitmask, nil
}

func (buf *Reader) ensureInBounds(numBits uint) error {
	if buf.currentBit+numBits > buf.totalBits {
		return fmt.Errorf("bitbuf attempt oob read by %d bits", (buf.currentBit+numBits)-buf.totalBits)
	}
	return nil
}

// NewReader returns a new Bitbuf reader.
func NewReader(data []byte) *Reader {
	return &Reader{
		internalBuffer: *bytes.NewBuffer(data),
		totalBits:      uint(len(data) * 8),
		currentBit:     0,
	}
}
