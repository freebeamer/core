package binfile

import (
	"encoding/binary"
	"fmt"
	"os"
)

func Load(path string) (*Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load binary %q: %w", path, err)
	}
	image, err := newImage(data, path)
	if err != nil {
		return nil, fmt.Errorf("load binary %q: %w", path, err)
	}
	return image, nil
}

func byteOrder(endian Endian) (binary.ByteOrder, error) {
	switch endian {
	case BigEndian:
		return binary.BigEndian, nil
	case LittleEndian:
		return binary.LittleEndian, nil
	default:
		return nil, ErrInvalidEndian
	}
}

func (i *Image) ReadUint8(offset uint64) (uint8, error) {
	b, err := i.Read(offset, 1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}
func (i *Image) ReadInt8(offset uint64) (int8, error) {
	v, err := i.ReadUint8(offset)
	return int8(v), err
}
func (i *Image) ReadUint16(offset uint64, endian Endian) (uint16, error) {
	order, err := byteOrder(endian)
	if err != nil {
		return 0, err
	}
	b, err := i.Read(offset, 2)
	if err != nil {
		return 0, err
	}
	return order.Uint16(b), nil
}
func (i *Image) ReadInt16(offset uint64, endian Endian) (int16, error) {
	v, err := i.ReadUint16(offset, endian)
	return int16(v), err
}
func (i *Image) ReadUint32(offset uint64, endian Endian) (uint32, error) {
	order, err := byteOrder(endian)
	if err != nil {
		return 0, err
	}
	b, err := i.Read(offset, 4)
	if err != nil {
		return 0, err
	}
	return order.Uint32(b), nil
}
func (i *Image) ReadInt32(offset uint64, endian Endian) (int32, error) {
	v, err := i.ReadUint32(offset, endian)
	return int32(v), err
}
