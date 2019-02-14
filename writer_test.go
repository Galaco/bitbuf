package bitbuf

import (
	"reflect"
	"testing"
)

func TestNewWriter(t *testing.T) {
	if reflect.TypeOf(NewWriter(32)) != reflect.TypeOf(&Writer{}) {
		t.Errorf("unexpect type. Expected: %s, but received: %s", reflect.TypeOf(NewWriter(32)), reflect.TypeOf(&Writer{}))
	}
}

func TestWriter_WriteInt8(t *testing.T) {
	sut := NewWriter(8)

	expected := []byte{124, 64, 124, 76, 1, 24, 76, 9}

	for i := 0; i < len(expected); i++ {
		if err := sut.WriteInt8(int8(expected[i])); err != nil {
			t.Error(err)
		}
	}

	for i, b := range sut.Data() {
		if b != expected[i] {
			t.Errorf("unexpected byte at position %d. expected %d, but received %d", int(i), uint8(expected[i]), uint8(b))
		}
	}
}

func TestWriter_WriteInt32(t *testing.T) {
	sut := NewWriter(8)

	expected := []int32{212345, -456356}
	expectedBytes := []byte{121, 61, 3, 0, 92, 9, 249, 255}

	for i := 0; i < len(expected); i++ {
		if err := sut.WriteInt32(expected[i]); err != nil {
			t.Error(err)
		}
	}

	for i, b := range sut.Data() {
		if b != expectedBytes[i] {
			t.Errorf("unexpected byte at position %d. expected %d, but received %d", int(i), uint8(expectedBytes[i]), uint8(b))
		}
	}
}

func TestWriter_MultipleWrite(t *testing.T) {

	sut := NewWriter(5)

	expectedBytes := []byte{124, 121, 61, 3, 0}

	if err := sut.WriteInt8(124); err != nil {
		t.Error(err)
	}
	if err := sut.WriteInt32(212345); err != nil {
		t.Error(err)
	}

	for i, b := range sut.Data() {
		if b != expectedBytes[i] {
			t.Errorf("unexpected byte at position %d. expected %d, but received %d", int(i), uint8(expectedBytes[i]), uint8(b))
		}
	}
}
