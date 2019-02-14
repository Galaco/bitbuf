package bitbuf

import (
	"bytes"
	"encoding/binary"
)

func bytesToUint32(data []byte) (ret uint32, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, binary.LittleEndian, &ret)

	return ret, err
}

func bytesToInt64(data []byte) (ret int64, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, binary.LittleEndian, &ret)

	return ret, err
}

func bytesToFloat32(data []byte) (ret float32, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, binary.LittleEndian, &ret)

	return ret, err
}

func bytesToFloat64(data []byte) (ret float64, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, binary.LittleEndian, &ret)

	return ret, err
}
