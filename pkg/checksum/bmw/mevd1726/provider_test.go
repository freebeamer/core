package mevd1726

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/freebeamer/core/pkg/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/checksum"
	"github.com/freebeamer/core/pkg/xdf"
)

func privateFixtures(t *testing.T) ([]byte, *types.MapDefinition) {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..", "testdata", "private")
	binPath := filepath.Join(root, "00000FEF81A001_original.bin")
	xdfPath := filepath.Join(root, "00000FEF81A001.xdf")
	image, err := os.ReadFile(binPath)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("private BMW BIN fixture not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	definition, err := xdf.ImportMapDefinition(xdfPath)
	if err != nil {
		t.Fatal(err)
	}
	return image, definition
}

func TestStock75P9EJ0BVerifiesAllElevenStructures(t *testing.T) {
	image, _ := privateFixtures(t)
	provider := New()
	detection := provider.Detect(image)
	if detection.Confidence != types.ChecksumDetectionHigh {
		t.Fatalf("detection=%#v", detection)
	}
	results, err := provider.Verify(image)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 11 || !checksum.AllValid(results) {
		t.Fatalf("results=%#v", results)
	}
}

func TestCalibrationMutationFailsAndCorrectionMatchesIndependentOracle(t *testing.T) {
	original, definition := privateFixtures(t)
	path := filepath.Join(t.TempDir(), "original.bin")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	parameter, err := calibration.FindParameter(definition, "Performance gauge scaling")
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := calibration.New(definition, image)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.SetCell(*parameter, 0, 0, 1.2); err != nil {
		t.Fatal(err)
	}
	modified := image.Bytes()
	provider := New()
	before, err := provider.Verify(modified)
	if err != nil {
		t.Fatal(err)
	}
	invalid := 0
	for _, result := range before {
		if !result.Valid {
			invalid++
			if result.Name != "block-0x60-1-ADD16" || result.Calculated != 0xcafeb00a {
				t.Fatalf("unexpected invalid result=%#v", result)
			}
		}
	}
	if invalid != 1 {
		t.Fatalf("invalid count=%d results=%#v", invalid, before)
	}
	correctedInput := append([]byte(nil), modified...)
	after, err := provider.Correct(correctedInput)
	if err != nil {
		t.Fatal(err)
	}
	if !checksum.AllValid(after) {
		t.Fatalf("after=%#v", after)
	}
	correctedCount := 0
	for _, result := range after {
		if result.Corrected {
			correctedCount++
		}
	}
	if correctedCount != 1 {
		t.Fatalf("corrected results=%#v", after)
	}
	wantHash := [32]byte{0x69, 0x8d, 0xc5, 0xeb, 0x0a, 0x18, 0x92, 0x34, 0x64, 0xd9, 0x78, 0xa4, 0x18, 0x80, 0x88, 0x37, 0x89, 0x35, 0x5c, 0x0d, 0x42, 0xf2, 0x50, 0x96, 0x94, 0x5e, 0x61, 0xb0, 0x80, 0xf6, 0xa7, 0x37}
	if got := sha256.Sum256(correctedInput); got != wantHash {
		t.Fatalf("corrected SHA-256=%x", got)
	}
	if !bytes.Equal(original, image.Original()) {
		t.Fatal("original image mutated")
	}
}

func TestCorrectionRefusesInvalidCodeCRC(t *testing.T) {
	image, _ := privateFixtures(t)
	image = append([]byte(nil), image...)
	image[0x15000] ^= 1
	before := append([]byte(nil), image...)
	_, err := New().Correct(image)
	if !errors.Is(err, ErrUnsafeCorrection) {
		t.Fatalf("error=%v", err)
	}
	if !bytes.Equal(image, before) {
		t.Fatal("failed correction mutated input")
	}
}

func TestDetectionRejectsCopiedMarkerWithoutManifest(t *testing.T) {
	image := make([]byte, 4*1024*1024)
	copy(image, "75P9EJ0B MEVD17.2.6")
	if got := New().Detect(image).Confidence; got != types.ChecksumDetectionNone {
		t.Fatalf("confidence=%d", got)
	}
}
