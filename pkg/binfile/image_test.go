package binfile

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func loadFixture(t *testing.T, data []byte) (*Image, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "original.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return image, path
}

func TestLoadRejectsEmpty(t *testing.T) {
	_, path := loadFixture(t, []byte{1})
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("Load error = %v, want ErrEmpty", err)
	}
}

func TestDefensiveCopiesAndImmutableOriginal(t *testing.T) {
	image, _ := loadFixture(t, []byte{1, 2, 3})
	original := image.Original()
	original[0] = 9
	working := image.Bytes()
	working[1] = 9
	if got := image.Original(); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("original mutated: %v", got)
	}
	if got := image.Bytes(); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("working mutated: %v", got)
	}
	if err := image.Write(1, []byte{8}); err != nil {
		t.Fatal(err)
	}
	if got := image.Original(); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("write mutated original: %v", got)
	}
}

func TestReadWriteBounds(t *testing.T) {
	image, _ := loadFixture(t, []byte{0, 1, 2, 3})
	for _, tc := range []struct {
		offset uint64
		length int
	}{{5, 0}, {3, 2}, {0, -1}, {^uint64(0), 1}} {
		if _, err := image.Read(tc.offset, tc.length); !errors.Is(err, ErrOutOfBounds) {
			t.Errorf("Read(%d,%d) = %v", tc.offset, tc.length, err)
		}
	}
	if err := image.Write(4, []byte{9}); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("Write error = %v", err)
	}
	if got, err := image.Read(4, 0); err != nil || len(got) != 0 {
		t.Fatalf("zero-length read = %v, %v", got, err)
	}
}

func TestTypedReadsAndWrites(t *testing.T) {
	image, _ := loadFixture(t, make([]byte, 16))
	if err := image.WriteUint8(0, 0xfe); err != nil {
		t.Fatal(err)
	}
	if err := image.WriteInt8(1, -2); err != nil {
		t.Fatal(err)
	}
	if err := image.WriteUint16(2, 0x1234, BigEndian); err != nil {
		t.Fatal(err)
	}
	if err := image.WriteInt16(4, -3, LittleEndian); err != nil {
		t.Fatal(err)
	}
	if err := image.WriteUint32(6, 0x12345678, LittleEndian); err != nil {
		t.Fatal(err)
	}
	if err := image.WriteInt32(10, -4, BigEndian); err != nil {
		t.Fatal(err)
	}

	if got, _ := image.ReadUint8(0); got != 0xfe {
		t.Errorf("uint8 = %#x", got)
	}
	if got, _ := image.ReadInt8(1); got != -2 {
		t.Errorf("int8 = %d", got)
	}
	if got, _ := image.ReadUint16(2, BigEndian); got != 0x1234 {
		t.Errorf("uint16 = %#x", got)
	}
	if got, _ := image.ReadInt16(4, LittleEndian); got != -3 {
		t.Errorf("int16 = %d", got)
	}
	if got, _ := image.ReadUint32(6, LittleEndian); got != 0x12345678 {
		t.Errorf("uint32 = %#x", got)
	}
	if got, _ := image.ReadInt32(10, BigEndian); got != -4 {
		t.Errorf("int32 = %d", got)
	}
	if !bytes.Equal(image.Bytes()[2:4], []byte{0x12, 0x34}) {
		t.Errorf("big-endian bytes = %x", image.Bytes()[2:4])
	}
	if !bytes.Equal(image.Bytes()[6:10], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Errorf("little-endian bytes = %x", image.Bytes()[6:10])
	}
	if _, err := image.ReadUint16(0, Endian(99)); !errors.Is(err, ErrInvalidEndian) {
		t.Errorf("invalid endian error = %v", err)
	}
}

func TestDiffAndReset(t *testing.T) {
	image, _ := loadFixture(t, []byte{0, 1, 2, 3})
	if err := image.Write(1, []byte{9, 8}); err != nil {
		t.Fatal(err)
	}
	want := []types.BinaryChange{{Offset: 1, Original: 1, Current: 9}, {Offset: 2, Original: 2, Current: 8}}
	if got := image.Diff(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Diff = %#v", got)
	}
	image.Reset()
	if len(image.Diff()) != 0 || !bytes.Equal(image.Bytes(), image.Original()) {
		t.Fatal("Reset did not restore image")
	}
	if _, err := DiffBytes([]byte{1}, []byte{1, 2}); !errors.Is(err, ErrSizeMismatch) {
		t.Errorf("DiffBytes error = %v", err)
	}
}

func TestSaveAsPreservesBytesAndRefusesOverwrite(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	image, input := loadFixture(t, data)
	output := filepath.Join(t.TempDir(), "copy.bin")
	if err := image.SaveAs(output); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(output)
	if err != nil {
		t.Fatal(err)
	}
	if image.SHA256() != reloaded.SHA256() || image.SHA256() != sha256.Sum256(data) {
		t.Fatal("saved digest differs")
	}
	if err := image.SaveAs(input); err == nil {
		t.Fatal("SaveAs overwrote input")
	}
	if err := image.SaveAs(output); err == nil {
		t.Fatal("SaveAs overwrote existing output")
	}
}

func TestSaveBytesWritesArbitraryDataAndRefusesOverwrite(t *testing.T) {
	// SaveBytes is the primitive SaveAs uses, exposed so a caller can save a
	// byte slice (a checksum-corrected clone) without mutating an Image's
	// own working buffer to do it.
	corrected := []byte{9, 9, 9, 9}
	output := filepath.Join(t.TempDir(), "corrected.bin")
	input := filepath.Join(t.TempDir(), "original.bin")
	if err := os.WriteFile(input, []byte{1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveBytes(output, corrected, input); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, corrected) {
		t.Fatalf("saved bytes = %v, want %v", saved, corrected)
	}
	if err := SaveBytes(input, corrected, input); err == nil {
		t.Fatal("SaveBytes overwrote the original path")
	}
	if err := SaveBytes(output, corrected, input); err == nil {
		t.Fatal("SaveBytes overwrote an existing output file")
	}
}

func TestPrivateBMWFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "private", "00000FEF81A001_original.bin")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Skip("private BMW fixture not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	image, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if image.Size() != 4_194_304 {
		t.Fatalf("size = %d", image.Size())
	}
	want := [32]byte{0x65, 0x09, 0xdc, 0x52, 0x57, 0x94, 0x80, 0x91, 0x19, 0x4c, 0xa9, 0x25, 0x7b, 0x0d, 0x57, 0x63, 0xdc, 0xc6, 0x7c, 0xa0, 0x14, 0xe1, 0x8d, 0xf4, 0xd6, 0x05, 0xd4, 0x19, 0x73, 0x8a, 0xad, 0xc8}
	if got := image.SHA256(); got != want {
		t.Fatalf("sha256 = %x", got)
	}
}
