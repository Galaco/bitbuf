// Package bitbuf reads and writes little-endian, least-significant-bit-first
// bit streams, as used by the Source Engine (bf_read / bf_write) for network
// messages, demos and other packed data.
//
// Bits are packed least-significant bit first into bytes, which is equivalent
// to the engine's little-endian 32-bit word packing. Besides fixed-width
// integers, floats, strings and raw bytes, the engine's specialised encodings
// are reproduced bit-for-bit: UBitVar, protobuf-style varints, world
// coordinates, normals and angles.
//
// Reads past the end of a buffer do not panic and do not return per-call
// errors. As in the engine, the reader is marked as overflowed, its cursor
// moves to the end and zero values are returned; writers behave the same way
// when they reach their limit. Decode a whole structure, then check Err once:
//
//	r := bitbuf.NewReader(data)
//	id := r.ReadBits(6)
//	name := r.ReadString()
//	if err := r.Err(); err != nil {
//		return err
//	}
package bitbuf

import (
	"fmt"
	"io"
)

// ErrOverflow is reported when a read or write runs past the end of a buffer.
// It wraps io.ErrUnexpectedEOF.
var ErrOverflow = fmt.Errorf("bitbuf: buffer overflow: %w", io.ErrUnexpectedEOF)

// Coordinate and normal encoding parameters (coordsize.h).
const (
	CoordIntegerBits          = 14
	CoordFractionalBits       = 5
	CoordDenominator          = 1 << CoordFractionalBits
	CoordResolution           = 1.0 / CoordDenominator
	CoordIntegerBitsMP        = 11
	CoordFractionalBitsMPLowP = 3
	CoordDenominatorLowP      = 1 << CoordFractionalBitsMPLowP
	CoordResolutionLowP       = 1.0 / CoordDenominatorLowP
	NormalFractionalBits      = 11
	NormalDenominator         = (1 << NormalFractionalBits) - 1
	NormalResolution          = 1.0 / NormalDenominator
	maxVarInt32Bytes          = 5
	maxVarInt64Bytes          = 10
)

// BitsToBytes returns the number of bytes needed to hold n bits.
func BitsToBytes(n int) int {
	return (n + 7) >> 3
}
