package bitbuf

import "bytes"

func NewWriter(length int) *Reader {
	return & Reader{
		internalBuffer: *bytes.NewBuffer(make([]byte, length)),
		totalBits:		  uint(length * 8),
		currentBit:       0,
	}
}
