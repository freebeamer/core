// Package binfile provides an immutable-original binary image with a separate
// editable working copy.
package binfile

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/freebeamer/core/pkg/types"
)

type Endian int

const (
	BigEndian Endian = iota
	LittleEndian
)

var (
	ErrEmpty         = errors.New("binary image is empty")
	ErrOutOfBounds   = errors.New("binary access out of bounds")
	ErrInvalidEndian = errors.New("invalid byte order")
	ErrSizeMismatch  = errors.New("binary image sizes differ")
)

type Image struct {
	original []byte
	working  []byte
	path     string
}

func newImage(data []byte, path string) (*Image, error) {
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	original := append([]byte(nil), data...)
	return &Image{original: original, working: append([]byte(nil), data...), path: path}, nil
}

func (i *Image) Size() uint64              { return uint64(len(i.working)) }
func (i *Image) Path() string              { return i.path }
func (i *Image) Original() []byte          { return append([]byte(nil), i.original...) }
func (i *Image) Bytes() []byte             { return append([]byte(nil), i.working...) }
func (i *Image) SHA256() [sha256.Size]byte { return sha256.Sum256(i.working) }

func (i *Image) bounds(offset uint64, length int) (int, int, error) {
	if length < 0 || offset > uint64(len(i.working)) {
		return 0, 0, fmt.Errorf("%w: offset %#x length %d image size %#x", ErrOutOfBounds, offset, length, len(i.working))
	}
	start := int(offset)
	if length > len(i.working)-start {
		return 0, 0, fmt.Errorf("%w: offset %#x length %d image size %#x", ErrOutOfBounds, offset, length, len(i.working))
	}
	return start, start + length, nil
}

func (i *Image) Read(offset uint64, length int) ([]byte, error) {
	start, end, err := i.bounds(offset, length)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), i.working[start:end]...), nil
}

func (i *Image) Write(offset uint64, data []byte) error {
	start, end, err := i.bounds(offset, len(data))
	if err != nil {
		return err
	}
	copy(i.working[start:end], data)
	return nil
}

func (i *Image) Diff() []types.BinaryChange {
	var changes []types.BinaryChange
	for offset, original := range i.original {
		if current := i.working[offset]; current != original {
			changes = append(changes, types.BinaryChange{Offset: uint64(offset), Original: original, Current: current})
		}
	}
	return changes
}

func (i *Image) Reset() { copy(i.working, i.original) }
