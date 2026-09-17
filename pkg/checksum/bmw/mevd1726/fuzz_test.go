package mevd1726

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func FuzzVerifyCorruptedFirmware(f *testing.F) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "private", "00000FEF81A001_original.bin")
	original, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		f.Skip("private BMW BIN fixture not installed")
	}
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []struct {
		offset uint32
		mask   byte
	}{
		{0, 1},
		{0x18171c, 0x0a},
		{0x1ffbfc, 0xff},
		{uint32(len(original) - 1), 1},
		{0, 0},
	} {
		f.Add(seed.offset, seed.mask)
	}

	provider := New()
	f.Fuzz(func(t *testing.T, offset uint32, mask byte) {
		candidate := append([]byte(nil), original...)
		candidate[int(offset)%len(candidate)] ^= mask
		before := append([]byte(nil), candidate...)
		results, err := provider.Verify(candidate)
		if !bytes.Equal(candidate, before) {
			t.Fatal("Verify mutated its input")
		}
		if err == nil && len(results) != 11 {
			t.Fatalf("Verify returned %d results, want 11", len(results))
		}
	})
}
