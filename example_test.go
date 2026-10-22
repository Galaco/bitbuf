package bitbuf_test

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/galaco/bitbuf/v2"
)

func ExampleNewReader() {
	var data bytes.Buffer
	binary.Write(&data, binary.LittleEndian, struct {
		A byte
		B int16
		C float32
	}{32, 8375, 2106.32})

	r := bitbuf.NewReader(data.Bytes())
	fmt.Println(r.ReadUint8())
	fmt.Println(r.ReadInt16())
	fmt.Println(r.ReadFloat32())
	fmt.Println(r.Err())
	fmt.Println(r.ReadUint8(), r.Err())
	// Output:
	// 32
	// 8375
	// 2106.32
	// <nil>
	// 0 bitbuf: buffer overflow: unexpected EOF
}

// Fields narrower than a byte, packed least-significant bit first.
func ExampleReader_ReadBits() {
	r := bitbuf.NewReader([]byte{0x5D, 0xA8})
	kind := r.ReadBits(3)
	flag := r.ReadBit()
	value := r.ReadBits(12)
	fmt.Println(kind, flag, value)
	// Output: 5 true 2693
}

func ExampleWriter_WriteBits() {
	w := bitbuf.NewWriter(2)
	w.WriteBits(5, 3)
	w.WriteBit(true)
	w.WriteBits(2693, 12)
	fmt.Printf("%#x\n", w.Bytes())
	// Output: 0x5da8
}

// A payload with a length in bits is read through a bounded sub-reader, so
// decoding it cannot run into the data that follows.
func ExampleReader_SubReader() {
	w := bitbuf.NewWriter(0)
	w.WriteBits(11, 5) // payload length in bits
	w.WriteBits('h', 8)
	w.WriteBits(1, 3)
	w.WriteBit(true) // data after the payload

	r := w.Reader()
	payload := r.SubReader(int(r.ReadBits(5)))
	fmt.Println(payload.ReadBits(8), payload.ReadBits(3), payload.BitsLeft())
	fmt.Println(r.ReadBit())
	// Output:
	// 104 1 0
	// true
}

// The Source Engine's variable-width integers and coordinates.
func ExampleWriter_WriteUBitVar() {
	w := bitbuf.NewWriter(0)
	w.WriteUBitVar(300)
	w.WriteVarInt32(300)
	w.WriteBitCoord(-12.5)

	r := w.Reader()
	fmt.Println(r.ReadUBitVar(), r.ReadVarInt32(), r.ReadBitCoord())
	// Output: 300 300 -12.5
}
