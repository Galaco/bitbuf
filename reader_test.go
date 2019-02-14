package bitbuf

import (
	"reflect"
	"testing"
)

func TestNewReader(t *testing.T) {
	if reflect.TypeOf(NewReader(make([]byte, 0))) != reflect.TypeOf(&Reader{}) {
		t.Error("unexpected type returned")
	}
}

func TestReader_ReadBits(t *testing.T) {
	t.Skip()
}

func TestReader_ReadByte(t *testing.T) {
	sut := NewReader(getTestBytes())

	expected := byte(32)
	if val, err := sut.ReadByte(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %b, but received: %b", expected, val)
		}
	}
}

func TestReader_ReadBytes(t *testing.T) {
	sut := NewReader(getTestBytes())

	sut.ReadByte()
	sut.ReadInt16()
	sut.ReadFloat32()
	sut.ReadInt64()

	expected := []byte{84, 12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 12, 13, 54, 1, 143, 234, 5, 56, 1, 2}
	if val, err := sut.ReadBytes(32); err != nil {
		if err != nil {
			t.Error(err)
		}

		for i := range val {
			if val[i] != expected[i] {
				t.Errorf("expected: %b, but received: %b", expected[i], val[i])
			}
		}
	}
}

func TestReader_ReadFloat32(t *testing.T) {
	sut := NewReader(getTestBytes())

	sut.ReadByte()
	sut.ReadInt16()

	expected := float32(2106.3212345)
	if val, err := sut.ReadFloat32(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %f, but received: %f", expected, val)
		}
	}
}

func TestReader_ReadFloat64(t *testing.T) {
	sut := NewReader(getTestBytes())

	sut.ReadByte()
	sut.ReadInt16()
	sut.ReadFloat32()
	sut.ReadInt64()
	sut.ReadBytes(32)
	sut.ReadUint8()

	expected := float64(-756351.123)
	if val, err := sut.ReadFloat64(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %f, but received: %f", expected, val)
		}
	}
}

func TestReader_ReadInt8(t *testing.T) {
	sut := NewReader(getTestBytes())

	sut.ReadByte()
	sut.ReadInt16()
	sut.ReadFloat32()
	sut.ReadInt64()
	sut.ReadBytes(32)
	sut.ReadUint8()
	sut.ReadFloat64()

	expected := int8(-57)
	if val, err := sut.ReadInt8(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %d, but received: %d", expected, val)
		}
	}
}

func TestReader_ReadInt16(t *testing.T) {
	sut := NewReader(getTestBytes())

	sut.ReadByte()

	expected := int16(8375)
	if val, err := sut.ReadInt16(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %d, but received: %d", expected, val)
		}
	}
}

func TestReader_ReadInt32(t *testing.T) {
	t.Skip()
}

func TestReader_ReadInt64(t *testing.T) {
	sut := NewReader(getTestBytes())

	sut.ReadByte()
	sut.ReadInt16()
	sut.ReadFloat32()

	expected := int64(5635455352)
	if val, err := sut.ReadInt64(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %d, but received: %d", expected, val)
		}
	}
}

func TestReader_ReadString(t *testing.T) {
	t.Skip()
}

func TestReader_ReadUint8(t *testing.T) {
	t.Skip()
}

func TestReader_ReadUint16(t *testing.T) {
	t.Skip()
}

func TestReader_ReadUint32(t *testing.T) {

	sut := NewReader(getTestBytes())

	sut.ReadByte()
	sut.ReadInt16()
	sut.ReadFloat32()
	sut.ReadInt64()
	sut.ReadBytes(32)
	sut.ReadUint8()
	sut.ReadFloat64()
	sut.ReadInt8()

	expected := uint32(12645123)
	if val, err := sut.ReadUint32(); err != nil && val != expected {
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected: %d, but received: %d", expected, val)
		}
	}
}

func TestReader_ReadUint64(t *testing.T) {
	t.Skip()
}

func getTestBytes() []byte {
	return []byte{
		32,
		183, 32,
		36, 165, 3, 69,
		120, 57, 230, 79, 1, 0, 0, 0,
		84, 12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 12, 13, 54, 1, 143, 234, 5, 56, 1, 2,
		213,
		35, 219, 249, 62, 254, 20, 39, 193,
		199,
		3, 243, 192, 0,
	}
}
