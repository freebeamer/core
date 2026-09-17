package binfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (i *Image) WriteUint8(offset uint64, value uint8) error { return i.Write(offset, []byte{value}) }
func (i *Image) WriteInt8(offset uint64, value int8) error   { return i.WriteUint8(offset, uint8(value)) }
func (i *Image) WriteUint16(offset uint64, value uint16, endian Endian) error {
	order, err := byteOrder(endian)
	if err != nil {
		return err
	}
	b := make([]byte, 2)
	order.PutUint16(b, value)
	return i.Write(offset, b)
}
func (i *Image) WriteInt16(offset uint64, value int16, endian Endian) error {
	return i.WriteUint16(offset, uint16(value), endian)
}
func (i *Image) WriteUint32(offset uint64, value uint32, endian Endian) error {
	order, err := byteOrder(endian)
	if err != nil {
		return err
	}
	b := make([]byte, 4)
	order.PutUint32(b, value)
	return i.Write(offset, b)
}
func (i *Image) WriteInt32(offset uint64, value int32, endian Endian) error {
	return i.WriteUint32(offset, uint32(value), endian)
}

func (i *Image) SaveAs(path string) error {
	return SaveBytes(path, i.working, i.path)
}

// SaveBytes writes data to a new file at path, refusing to overwrite
// originalPath (if non-empty) and cleaning up a partial file if the write
// fails partway through. It is the primitive Image.SaveAs uses, exported so
// a caller that must save a byte slice it deliberately kept out of an
// Image's working buffer (a checksum-corrected clone, for example) gets the
// exact same original-path and partial-write safety.
func SaveBytes(path string, data []byte, originalPath string) error {
	if path == "" {
		return errors.New("save binary: output path is empty")
	}
	if originalPath != "" {
		input, err := filepath.Abs(originalPath)
		if err != nil {
			return fmt.Errorf("resolve input path: %w", err)
		}
		output, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
		if input == output {
			return errors.New("save binary: refusing to overwrite input")
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("save binary %q: %w", path, err)
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("save binary %q: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("save binary %q: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save binary %q: %w", path, err)
	}
	ok = true
	return nil
}
