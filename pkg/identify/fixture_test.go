package identify

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func TestPrivateBMWFirmwareIdentity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "private", "00000FEF81A001_original.bin")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("private BMW BIN fixture not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	identity := Identify(data)
	if identity.ECUVendor != "Bosch" || identity.ECUFamily != "MEVD17.2.6" || identity.Engine != "BMW N55" || identity.Confidence != types.IdentificationHigh {
		t.Fatalf("identity=%#v", identity)
	}
}
