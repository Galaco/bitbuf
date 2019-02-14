[![GoDoc](https://godoc.org/github.com/Galaco/bitbuf?status.svg)](https://godoc.org/github.com/Galaco/bitbuf)
[![Go report card](https://goreportcard.com/badge/github.com/galaco/bitbuf)](https://goreportcard.com/badge/github.com/galaco/bitbuf)
[![Build Status](https://travis-ci.com/Galaco/bitbuf.svg?branch=master)](https://travis-ci.com/Galaco/bitbuf)

# bitbuf

A readable bitstream. Create from a byte slice, and read through the stream
bit by bit.

Supports the following read types:
* `byte`, `[]byte`
* `int8`, `int16`, `int32`, `int64`
* `uint8`, `uint16`, `uint32`, `uint64`
* `float32`, `float64`
* `string` (of known length, or until null terminator)
* `bits` (returned as `[]byte`


### Usage
```go
package main

import (
	"bytes"
	"encoding/binary"
	"github.com/galaco/bitbuf"
	"log"
)

type Foo struct {
	A byte
	B int16
	C float32
	D int64
	E [32]byte
	F uint8
	G float64
	H int8
	I uint32
}

func main() {
	dataBuffer := &bytes.Buffer{}
	f := Foo{
		A: 32,
		B: 8375,
		C: 2106.3212345,
		D: 5635455352,
		E: [32]byte{84,12,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,12,13,54,1,143,234,5,56,1,2},
		F: 213,
		G: -756351.123,
		H: -57,
		I: 12645123,
	}

	binary.Write(dataBuffer, binary.LittleEndian, f)

	buf := bitbuf.NewReader(dataBuffer.Bytes())
	log.Println(buf.ReadByte())
	log.Println(buf.ReadInt16())
	log.Println(buf.ReadFloat32())
	log.Println(buf.ReadInt64())
	log.Println(buf.ReadBytes(32))
	log.Println(buf.ReadUint8())
	log.Println(buf.ReadFloat64())
	log.Println(buf.ReadInt8())
	log.Println(buf.ReadUint32())
}
```
