package calibration_test

// A cross-fixture import/decode invariant harness (export plan item 5, I1).
// For every committed XDF and A2L fixture it converts to mapdef, decodes every
// parameter against a definition-sized image, and asserts invariants that hold
// regardless of an external oracle:
//
//   - decode succeeds and is deterministic for every supported parameter;
//   - every raw and physical value is finite (decode rejects non-finite);
//   - an edit round-trips: setting a cell to its own decoded value stores a
//     value that reads back equal and is idempotent (no drift on re-apply).
//
// Where a conversion is decode-only (non-invertible), the edit invariant is
// skipped, not failed. This protects import quality across the whole fixture
// set and is the test bed a future exporter's round-trip checks build on. It
// deliberately does not reimplement decoding as a second "oracle"; the
// byte-level XDF oracle lives in pkg/xdf's own conversion test.

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/a2l"
	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/xdf"
)

func convertedFixtures(t *testing.T) map[string]*types.MapDefinition {
	t.Helper()
	out := map[string]*types.MapDefinition{}
	for _, path := range globFixtures(t, filepath.Join("..", "xdf", "testdata", "*.xdf")) {
		src, err := xdf.Load(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		doc, err := (xdf.Converter{}).Convert(src, path)
		if err != nil {
			t.Fatalf("convert %s: %v", path, err)
		}
		out[filepath.Base(path)] = doc
	}
	for _, path := range globFixtures(t, filepath.Join("..", "a2l", "testdata", "*.a2l")) {
		src, err := a2l.Load(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		doc, err := (a2l.Converter{}).Convert(src, path)
		if err != nil {
			t.Fatalf("convert %s: %v", path, err)
		}
		out[filepath.Base(path)] = doc
	}
	if len(out) == 0 {
		t.Fatal("no fixtures found")
	}
	return out
}

func globFixtures(t *testing.T, pattern string) []string {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// imageForDefinition builds a zero-filled image large enough to cover every
// addressed layout in the document.
func imageForDefinition(t *testing.T, doc *types.MapDefinition) *binfile.Image {
	t.Helper()
	size := requiredImageSize(doc)
	path := filepath.Join(t.TempDir(), "image.bin")
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

func dataTypeBytes(dataType string) int {
	switch dataType {
	case "uint8", "int8":
		return 1
	case "uint16", "int16":
		return 2
	case "uint32", "int32", "float32":
		return 4
	case "float64":
		return 8
	default:
		return 0 // "unknown": not decoded
	}
}

func layoutEnd(layout types.DataLayout, count, baseOffset int) int {
	if layout.Address == nil {
		return 0
	}
	elem := dataTypeBytes(layout.DataType)
	if elem == 0 || count <= 0 {
		return 0
	}
	pitch := elem
	for _, stride := range layout.StrideBits {
		if stride > 0 && stride/8 > pitch {
			pitch = stride / 8
		}
	}
	// A negative base offset only shrinks the real offset, so using the raw
	// address overestimates the needed size, which is safe.
	base := int(*layout.Address)
	if baseOffset > 0 {
		base += baseOffset
	}
	return base + count*pitch
}

func requiredImageSize(doc *types.MapDefinition) int {
	max := 4096 // floor
	base := int(doc.Memory.BaseOffset)
	for i := range doc.Parameters {
		p := doc.Parameters[i]
		count := 1
		for _, dim := range p.Layout.Dimensions {
			count *= dim
		}
		if end := layoutEnd(p.Layout, count, base); end > max {
			max = end
		}
		for _, axis := range p.Axes {
			if axis.Layout == nil {
				continue
			}
			if end := layoutEnd(*axis.Layout, axis.Count, base); end > max {
				max = end
			}
		}
	}
	return max + 16
}

var invertSkip = []error{
	expression.ErrNotInvertible, expression.ErrNonAffine,
	calibration.ErrLookupTableNotInvertible, calibration.ErrValueOutsideLookupTable,
	calibration.ErrValueNotInLookupTable,
}

func isSkippableEditError(err error) bool {
	for _, target := range invertSkip {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func TestFixtureDecodeInvariants(t *testing.T) {
	// Subtests run synchronously (no t.Parallel), so this counts how many
	// parameters actually exercised the edit round-trip. Guards the harness
	// against silently degrading to "every edit skipped".
	editsVerified := 0
	for name, doc := range convertedFixtures(t) {
		doc := doc
		t.Run(name, func(t *testing.T) {
			for _, parameter := range doc.Parameters {
				parameter := parameter
				t.Run(parameter.Name, func(t *testing.T) {
					// Determinism + finiteness: two fresh decodes must match
					// exactly, and decode rejects any non-finite value.
					a := decode(t, doc, parameter)
					b := decode(t, doc, parameter)
					if a == nil {
						return // explicitly unsupported parameter
					}
					if !reflect.DeepEqual(a, b) {
						t.Fatal("decode is not deterministic")
					}
					assertFinite(t, a)

					// Edit round-trip: set cell 0 to its own decoded value and
					// confirm the stored value reads back equal and is stable.
					decoder, _ := calibration.New(doc, imageForDefinition(t, doc))
					decoded, err := decoder.DecodeParameter(parameter)
					if err != nil {
						return
					}
					current := decoded.Z[0][0]
					result, err := decoder.SetIndex(parameter, 0, current)
					if err != nil {
						if isSkippableEditError(err) {
							return // decode-only conversion
						}
						t.Fatalf("set index 0 to decoded value %g: %v", current, err)
					}
					if !closeEnough(result.Actual, current) {
						t.Fatalf("round-trip drift: set %g, stored %g", current, result.Actual)
					}
					again, err := decoder.SetIndex(parameter, 0, result.Actual)
					if err != nil {
						t.Fatalf("re-apply stored value: %v", err)
					}
					if !reflect.DeepEqual(again.Bytes, result.Bytes) {
						t.Fatalf("edit not idempotent: %v then %v", result.Bytes, again.Bytes)
					}
					editsVerified++
				})
			}
		})
	}
	if editsVerified == 0 {
		t.Fatal("no parameter exercised the edit round-trip; the harness has gone vacuous")
	}
}

func decode(t *testing.T, doc *types.MapDefinition, parameter types.MapParameter) *types.DecodedTable {
	t.Helper()
	decoder, err := calibration.New(doc, imageForDefinition(t, doc))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(parameter)
	if err != nil {
		if errors.Is(err, calibration.ErrUnsupportedDataType) || errors.Is(err, calibration.ErrUnsupportedStride) {
			return nil // a parameter the importer marked unsupported
		}
		t.Fatalf("decode %q: %v", parameter.Name, err)
	}
	return decoded
}

func assertFinite(t *testing.T, decoded *types.DecodedTable) {
	t.Helper()
	check := func(values []float64, label string) {
		for _, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("%s has a non-finite value %v", label, v)
			}
		}
	}
	check(decoded.X, "x")
	check(decoded.Y, "y")
	for _, row := range decoded.Z {
		check(row, "z")
	}
}

func closeEnough(a, b float64) bool {
	diff := math.Abs(a - b)
	scale := math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	return diff <= 1e-6*scale
}
