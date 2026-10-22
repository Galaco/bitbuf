[![GoDoc](https://godoc.org/github.com/Galaco/bitbuf?status.svg)](https://pkg.go.dev/github.com/galaco/bitbuf/v2)
[![Go report card](https://goreportcard.com/badge/github.com/galaco/bitbuf)](https://goreportcard.com/badge/github.com/galaco/bitbuf)
[![CI](https://github.com/Galaco/bitbuf/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Galaco/bitbuf/actions/workflows/ci.yml)

# bitbuf

A reader and writer for little-endian, least-significant-bit-first bit streams, as used by the
Source Engine (`bf_read` / `bf_write`) for network messages, demos and other packed data.

```sh
go get github.com/galaco/bitbuf/v2
```

Supported:
* bit fields of 0–32 bits, signed or unsigned, at any alignment
* `bool`, `int8`–`int64`, `uint8`–`uint64`, `float32`, `float64`, raw bytes
* NUL-terminated strings
* the Source Engine encodings, bit-for-bit: UBitVar, protobuf varints (with zigzag), world
  coordinates (including the multiplayer variants), normals, normal vectors and angles
* readers limited to an exact number of bits, and sub-readers over part of a stream
* growable writers, optionally limited to a maximum size, with seeking and in-place patching

## Usage

```go
r := bitbuf.NewReader(data)
kind := r.ReadBits(6)
name := r.ReadString()
origin := r.ReadBitVec3Coord()
payload := r.SubReader(int(r.ReadBits(11))) // a payload whose length is given in bits
if err := r.Err(); err != nil {
	return err
}

w := bitbuf.NewWriter(64)
w.WriteBits(kind, 6)
w.WriteString(name)
w.WriteBitVec3Coord(origin)
out := w.Bytes()
```

Reads past the end of a buffer don't panic and don't return an error per call. As in the engine,
the reader is marked as overflowed, its cursor moves to the end and zero values are returned. A
limited writer drops the write that doesn't fit. Decode or encode a whole structure, then check
`Err` once. `ErrOverflow` wraps `io.ErrUnexpectedEOF`.

## Migrating from v1

v2 replaces per-call errors with the overflow flag above and adds the Source Engine encodings.

| v1 | v2 |
|---|---|
| `v, err := r.ReadUint32Bits(n)` | `v := r.ReadBits(n)`, then `r.Err()` |
| `r.ReadInt32Bits(n)` | `r.ReadSBits(n)` |
| `r.ReadOneBit()` | `r.ReadBit()` |
| `r.ReadByte()` | `r.ReadUint8()` |
| `r.ReadBits(n)` (`[]byte`) | `r.ReadBitsToBytes(n)` |
| `r.ReadBytes(n)` | `r.ReadBytes(n)` (takes `int`) |
| `r.ReadString(0)` | `r.ReadString()` |
| `r.ReadString(max)` | `r.ReadLimitedString(max)`: the whole string is consumed and the result truncated, so the following fields stay aligned |
| `r.Size()`, `r.BitsRead()` | `r.Len()`, `r.Pos()` |
| `r.Seek(bit)`, `r.Reset()` | `r.Seek(bit)`, `r.Seek(0)` |
| `bitbuf.NewWriter(n)` (fixed size) | `bitbuf.NewWriter(capacity)` (grows) or `bitbuf.NewLimitedWriter(n)` |
| `w.WriteUnsignedBitInt32(v, n)` | `w.WriteBits(v, n)` |
| `w.WriteSignedBitInt32(v, n)` | `w.WriteSBits(v, n)` |
| `w.WriteByte(b)` | `w.WriteUint8(b)` |
| `w.WriteString(s)` | `w.WriteString(s)` now appends the NUL terminator |
| `w.Data()` | `w.Bytes()` |
| `w.Seek(bit)` | `w.Seek(bit)`, or `w.WriteBitsAt(bit, v, n)` to patch without moving the cursor |

Writes no longer return errors; check `w.Err()` for limited writers. v2 requires Go 1.21.
