package bitbuf_test

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/galaco/bitbuf"
)

func ExampleNewReader() {
	var data bytes.Buffer
	binary.Write(&data, binary.LittleEndian, struct {
		A byte
		B int16
		C float32
	}{32, 8375, 2106.32})

	r := bitbuf.NewReader(data.Bytes())
	fmt.Println(r.ReadByte())
	fmt.Println(r.ReadInt16())
	fmt.Println(r.ReadFloat32())
	fmt.Println(r.ReadByte())
	// Output:
	// 32 <nil>
	// 8375 <nil>
	// 2106.32 <nil>
	// 0 EOF
}

// Fields narrower than a byte, packed least-significant bit first.
func ExampleReader_ReadUint32Bits() {
	r := bitbuf.NewReader([]byte{0x5D, 0xA8})
	kind, _ := r.ReadUint32Bits(3)
	flag := r.ReadOneBit()
	value, _ := r.ReadUint32Bits(12)
	fmt.Println(kind, flag, value)
	// Output: 5 true 2693
}

func ExampleWriter_WriteUnsignedBitInt32() {
	w := bitbuf.NewWriter(2)
	w.WriteUnsignedBitInt32(5, 3)
	w.WriteUnsignedBitInt32(1, 1)
	w.WriteUnsignedBitInt32(2693, 12)
	fmt.Printf("%#x\n", w.Data())
	// Output: 0x5da8
}
