package calibration

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/xdf"
)

func loadPrivateDefinition(t *testing.T) *types.MapDefinition {
	t.Helper()
	xdfPath := filepath.Join("..", "..", "testdata", "private", "00000FEF81A001.xdf")
	if _, err := os.Stat(xdfPath); errors.Is(err, os.ErrNotExist) {
		t.Skip("private BMW XDF fixture not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	definition, err := xdf.ImportMapDefinition(xdfPath)
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func TestEveryAddressedPrivateBMWMapFitsFourMiB(t *testing.T) {
	definition := loadPrivateDefinition(t)
	binPath := filepath.Join(t.TempDir(), "synthetic-4mib.bin")
	if err := os.WriteFile(binPath, make([]byte, 4*1024*1024), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(binPath)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := New(definition, image)
	if err != nil {
		t.Fatal(err)
	}
	failures := decoder.Validate()
	for _, failure := range failures {
		t.Error(failure)
	}
}

// These expected values were independently reproduced with KingAI TunerPro
// Exporter v3.6.0 at commit 18697503fd5b1208045f667affa7c50803f6f894.
func TestRepresentativePrivateBMWMapsMatchIndependentDecoder(t *testing.T) {
	definition := loadPrivateDefinition(t)
	data := make([]byte, 4*1024*1024)
	putUnsigned := func(address int, values []uint16) {
		for index, value := range values {
			binary.LittleEndian.PutUint16(data[address+index*2:], value)
		}
	}
	putSigned := func(address int, values []int16) {
		for index, value := range values {
			binary.LittleEndian.PutUint16(data[address+index*2:], uint16(value))
		}
	}
	putUnsigned(0x181770, []uint16{100, 200, 300, 400, 500, 600})
	putUnsigned(0x181794, []uint16{1000, 2000, 3000, 4000, 5000, 6000})
	signedValues := make([]int16, 36)
	for index := range signedValues {
		signedValues[index] = int16(-1800 + index*100)
	}
	putSigned(0x18171c, signedValues)
	putUnsigned(0x181f36, []uint16{2000})
	binPath := filepath.Join(t.TempDir(), "oracle-pattern.bin")
	if err := os.WriteFile(binPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(binPath)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := New(definition, image)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want [][]float64
	}{
		{"Zeitkonstante zum Abschalten des Massenstromregler Hub", [][]float64{{0.9765626000000001}}},
		{"Performance gauge scaling X (autogen)", [][]float64{{10, 20, 30, 40, 50, 60}}},
		{"Performance gauge scaling", [][]float64{{-180, -170, -160, -150, -140, -130}, {-120, -110, -100, -90, -80, -70}, {-60, -50, -40, -30, -20, -10}, {0, 10, 20, 30, 40, 50}, {60, 70, 80, 90, 100, 110}, {120, 130, 140, 150, 160, 170}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parameter, findErr := FindParameter(definition, test.name)
			if findErr != nil {
				t.Fatal(findErr)
			}
			decoded, decodeErr := decoder.DecodeParameter(*parameter)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if !reflect.DeepEqual(decoded.Z, test.want) {
				t.Fatalf("decoded Z = %#v, want %#v", decoded.Z, test.want)
			}
		})
	}
}

func TestEditPrivatePerformanceGaugeCell(t *testing.T) {
	definition := loadPrivateDefinition(t)
	binPath := filepath.Join(t.TempDir(), "original.bin")
	if err := os.WriteFile(binPath, make([]byte, 4*1024*1024), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(binPath)
	if err != nil {
		t.Fatal(err)
	}
	parameter, err := FindParameter(definition, "Performance gauge scaling")
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := New(definition, image)
	if err != nil {
		t.Fatal(err)
	}
	result, err := decoder.SetCell(*parameter, 0, 0, 1.23)
	if err != nil {
		t.Fatal(err)
	}
	if result.Raw != 12 || math.Abs(result.Actual-1.2) > 1e-12 || result.Offset != 0x18171c {
		t.Fatalf("result=%#v", result)
	}
	want := []types.BinaryChange{{Offset: 0x18171c, Original: 0, Current: 12}}
	if !reflect.DeepEqual(image.Diff(), want) {
		t.Fatalf("diff=%#v", image.Diff())
	}
	if image.Original()[0x18171c] != 0 {
		t.Fatal("original changed")
	}
	output := filepath.Join(t.TempDir(), "modified.bin")
	if err := image.SaveAs(output); err != nil {
		t.Fatal(err)
	}
	reloaded, err := binfile.Load(output)
	if err != nil {
		t.Fatal(err)
	}
	reloadedDecoder, err := New(definition, reloaded)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := reloadedDecoder.DecodeParameter(*parameter)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(decoded.Z[0][0]-1.2) > 1e-12 || decoded.Z[0][1] != 0 {
		t.Fatalf("reloaded Z=%#v", decoded.Z)
	}
}
