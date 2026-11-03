package xdf_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/xdf"
)

// Synthetic stride fixtures authored for this project. See
// docs/xdf-compatibility.md's stride evidence section.

func strideImage(t *testing.T, data []byte) *binfile.Image {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stride.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

func diagnosticCodes(document *types.MapDefinition) []string {
	var codes []string
	for _, diagnostic := range document.ImportDiagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

// curveWithAxis is a 1x2 8-bit Z at 0x10 with a 16-bit X axis at 0x00.
func curveWithAxis(major, minor int) *types.XDFDefinition {
	source := definitionWithZ(types.XDFEmbeddedData{Address: 0x10, ElementSizeBits: 8, RowCount: 1, ColumnCount: 2, Raw: map[string]string{"mmedaddress": "0x10"}})
	source.Tables[0].X = types.XDFAxis{ID: "x", Formula: "X", IndexCount: 2, Embedded: types.XDFEmbeddedData{
		Address: 0, ElementSizeBits: 16, MajorStrideBits: major, MinorStrideBits: minor, Raw: map[string]string{"mmedaddress": "0x0"},
	}}
	return source
}

// Bytes: 16-bit big-endian 1, then 0xAAAA filler, then 2; Z cells 5 and 6.
var strideBytes = []byte{0, 1, 0xAA, 0xAA, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 6}

func TestAxisStrideEqualToElementWidthIsContiguous(t *testing.T) {
	document, err := (xdf.Converter{}).Convert(curveWithAxis(16, 0), "stride.xdf")
	if err != nil {
		t.Fatal(err)
	}
	axis := document.Parameters[0].Axes["x"]
	if axis.Layout.DataType != "uint16" || !reflect.DeepEqual(axis.Layout.StrideBits, []int{0, 0}) {
		t.Fatalf("axis layout = %#v", axis.Layout)
	}
	if codes := diagnosticCodes(document); !reflect.DeepEqual(codes, []string{"xdf.contiguous-stride"}) {
		t.Fatalf("diagnostics = %v", codes)
	}
	decoder, err := calibration.New(document, strideImage(t, strideBytes))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(document.Parameters[0])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.X, []float64{1, 0xAAAA}) || !reflect.DeepEqual(decoded.Addresses.X, []uint64{0, 2}) {
		t.Fatalf("axis = %v at %v, want packed 16-bit reads", decoded.X, decoded.Addresses.X)
	}
}

func TestUnestablishedAxisStridesDoNotDecode(t *testing.T) {
	for _, test := range []struct {
		name         string
		major, minor int
	}{
		{"major-wider-than-element", 32, 0}, // readers disagree: step vs ignored
		{"major-narrower-than-element", 8, 0},
		{"non-zero-minor", 16, 16}, // readers disagree: packed vs padding
		{"minor-only", 0, 16},
	} {
		t.Run(test.name, func(t *testing.T) {
			document, err := (xdf.Converter{}).Convert(curveWithAxis(test.major, test.minor), "stride.xdf")
			if err != nil {
				t.Fatal(err)
			}
			axis := document.Parameters[0].Axes["x"]
			if axis.Layout.DataType != "unknown" || !reflect.DeepEqual(axis.Layout.StrideBits, []int{test.major, test.minor}) {
				t.Fatalf("axis layout = %#v, want unknown with the source stride preserved", axis.Layout)
			}
			if codes := diagnosticCodes(document); !reflect.DeepEqual(codes, []string{"xdf.unsupported-stride"}) {
				t.Fatalf("diagnostics = %v", codes)
			}
			decoder, err := calibration.New(document, strideImage(t, strideBytes))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decoder.DecodeParameter(document.Parameters[0]); !errors.Is(err, calibration.ErrUnsupportedDataType) {
				t.Fatalf("decode error = %v, want the axis to be refused rather than read with a guessed step", err)
			}
		})
	}
}

func TestTableStrideOnlyContiguousForOneDimensionalRuns(t *testing.T) {
	for _, test := range []struct {
		name       string
		rows, cols int
		major      int
		want       []int
		code       string
	}{
		{"single-row", 1, 3, 8, []int{0, 0}, "xdf.contiguous-stride"},
		{"single-column", 3, 1, 8, []int{0, 0}, "xdf.contiguous-stride"},
		{"two-dimensional", 2, 3, 8, []int{8, 0}, "xdf.unsupported-stride"}, // row pitch vs cell step
	} {
		t.Run(test.name, func(t *testing.T) {
			embedded := types.XDFEmbeddedData{Address: 0, ElementSizeBits: 8, RowCount: test.rows, ColumnCount: test.cols, MajorStrideBits: test.major, Raw: map[string]string{"mmedaddress": "0x0"}}
			document, err := (xdf.Converter{}).Convert(definitionWithZ(embedded), "stride.xdf")
			if err != nil {
				t.Fatal(err)
			}
			if got := document.Parameters[0].Layout.StrideBits; !reflect.DeepEqual(got, test.want) {
				t.Fatalf("stride = %v, want %v", got, test.want)
			}
			if codes := diagnosticCodes(document); !reflect.DeepEqual(codes, []string{test.code}) {
				t.Fatalf("diagnostics = %v", codes)
			}
			decoder, err := calibration.New(document, strideImage(t, []byte{1, 2, 3, 4, 5, 6}))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decoder.DecodeParameter(document.Parameters[0])
			if test.code == "xdf.unsupported-stride" {
				if !errors.Is(err, calibration.ErrUnsupportedStride) {
					t.Fatalf("decode error = %v, want ErrUnsupportedStride", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			flat := []float64{}
			for _, row := range decoded.Z {
				flat = append(flat, row...)
			}
			if !reflect.DeepEqual(flat, []float64{1, 2, 3}) {
				t.Fatalf("decoded = %v, want packed cells", decoded.Z)
			}
		})
	}
}

func TestVirtualAxisNegativeStrideStillSkipped(t *testing.T) {
	source := curveWithAxis(0, 0)
	source.Tables[0].X.Embedded = types.XDFEmbeddedData{ElementSizeBits: 8, MajorStrideBits: -32, Raw: map[string]string{}}
	document, err := (xdf.Converter{}).Convert(source, "stride.xdf")
	if err != nil {
		t.Fatal(err)
	}
	if axis := document.Parameters[0].Axes["x"]; axis.Layout != nil {
		t.Fatalf("unaddressed axis got a layout: %#v", axis.Layout)
	}
	if codes := diagnosticCodes(document); codes != nil {
		t.Fatalf("diagnostics = %v", codes)
	}
}
