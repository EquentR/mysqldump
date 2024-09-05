package mysqldump

import (
	"bytes"
	"encoding/binary"
)

const (
	DDL = iota
	DML
)

type Package struct {
	Type int32
	Len  uint32
	data []byte
}

func NewPackage(data []byte, typ int32) *Package {
	return &Package{
		Type: typ,
		Len:  uint32(len(data)),
		data: data,
	}
}

func (p *Package) Bytes() ([]byte, error) {
	var dataBuffer bytes.Buffer
	if err := binary.Write(&dataBuffer, binary.LittleEndian, p.Type); err != nil {
		return nil, err
	}
	if err := binary.Write(&dataBuffer, binary.LittleEndian, p.Len); err != nil {
		return nil, err
	}
	if _, err := dataBuffer.Write(p.data); err != nil {
		return nil, err
	}
	return dataBuffer.Bytes(), nil
}
