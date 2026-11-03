package a2l

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/mapdef"
)

// Synthetic fixtures for FLOAT32_IEEE / FLOAT64_IEEE, authored for this
// project; see docs/a2l-v0-plan.md's FLOAT32_IEEE / FLOAT64_IEEE evidence.

func floatImage(t *testing.T, data []byte) *binfile.Image {
	t.Helper()
	path := filepath.Join(t.TempDir(), "float.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

func TestConvertFloatScalarsDecodeAndEdit(t *testing.T) {
	source := syntheticSource(t, `
    /begin MOD_COMMON ""
      BYTE_ORDER MSB_LAST
      ALIGNMENT_FLOAT32_IEEE 4
      ALIGNMENT_FLOAT64_IEEE 4
    /end MOD_COMMON
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%8.4" "ratio"
    /end COMPU_METHOD
    /begin COMPU_METHOD CM.HALF
      "half"
      LINEAR "%8.4" "ratio"
      COEFFS_LINEAR 0.5 0
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL.F32
      FNC_VALUES 1 FLOAT32_IEEE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin RECORD_LAYOUT RL.F64
      FNC_VALUES 1 FLOAT64_IEEE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC Lambda
      "little-endian float32 from MOD_COMMON"
      VALUE
      0x10
      RL.F32
      0
      CM.IDENTICAL
      0.5 1.5
    /end CHARACTERISTIC
    /begin CHARACTERISTIC Gain
      "big-endian float64 override"
      VALUE
      0x20
      RL.F64
      0
      CM.HALF
      -1000 1000
      BYTE_ORDER MSB_FIRST
    /end CHARACTERISTIC`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if document.FormatVersion != mapdef.CurrentVersion || mapdef.Validate(document) != nil {
		t.Fatalf("document version %q invalid: %v", document.FormatVersion, mapdef.Validate(document))
	}
	lambda, gain := document.Parameters[0], document.Parameters[1]
	if lambda.Layout.DataType != "float32" || lambda.Layout.ByteOrder != "little" {
		t.Fatalf("lambda layout = %#v", lambda.Layout)
	}
	if gain.Layout.DataType != "float64" || gain.Layout.ByteOrder != "big" {
		t.Fatalf("gain layout = %#v", gain.Layout)
	}

	data := make([]byte, 0x28)
	binary.LittleEndian.PutUint32(data[0x10:], math.Float32bits(0.875))
	binary.BigEndian.PutUint64(data[0x20:], math.Float64bits(-12.5))
	image := floatImage(t, data)
	decoder, err := calibration.New(document, image)
	if err != nil {
		t.Fatal(err)
	}
	decodedLambda, err := decoder.DecodeParameter(lambda)
	if err != nil {
		t.Fatal(err)
	}
	decodedGain, err := decoder.DecodeParameter(gain)
	if err != nil {
		t.Fatal(err)
	}
	if decodedLambda.Z[0][0] != 0.875 || decodedGain.Raw.Z[0][0] != -12.5 || decodedGain.Z[0][0] != -6.25 {
		t.Fatalf("decoded lambda %v, gain raw %v physical %v", decodedLambda.Z, decodedGain.Raw.Z, decodedGain.Z)
	}

	if _, err := decoder.SetIndex(gain, 0, 3.3); err != nil {
		t.Fatal(err)
	}
	if got := math.Float64frombits(binary.BigEndian.Uint64(image.Bytes()[0x20:])); got != 6.6 {
		t.Fatalf("gain stored %v after edit, want 6.6 (3.3 / 0.5)", got)
	}
	if !reflect.DeepEqual(image.Bytes()[0x10:0x14], data[0x10:0x14]) {
		t.Fatal("editing gain touched lambda's bytes")
	}
}

func TestConvertFloatCurveWithFloatAxis(t *testing.T) {
	source := syntheticSource(t, `
    /begin MOD_COMMON ""
      BYTE_ORDER MSB_FIRST
    /end MOD_COMMON
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%8.2" "-"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL.CURVE.FLOAT
      NO_AXIS_PTS_X 1 UBYTE
      AXIS_PTS_X 2 FLOAT32_IEEE INDEX_INCR DIRECT
      FNC_VALUES 3 FLOAT64_IEEE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC Curve
      "float32 axis, float64 values"
      CURVE
      0x100
      RL.CURVE.FLOAT
      0
      CM.IDENTICAL
      -100 100
      /begin AXIS_DESCR
        STD_AXIS
        M.RPM
        CM.IDENTICAL
        3
        0 8000
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	curve := document.Parameters[0]
	axis := curve.Axes["x"]
	// Layout: 1-byte count, 3 x 4-byte axis points, then 3 x 8-byte values.
	if axis.Layout == nil || *axis.Layout.Address != 0x101 || axis.Layout.DataType != "float32" {
		t.Fatalf("axis layout = %#v", axis.Layout)
	}
	if *curve.Layout.Address != 0x10D || curve.Layout.DataType != "float64" {
		t.Fatalf("values layout = %#v", curve.Layout)
	}

	data := make([]byte, 0x125)
	for i, point := range []float32{1000, 2500.5, 6000} {
		binary.BigEndian.PutUint32(data[0x101+i*4:], math.Float32bits(point))
	}
	for i, value := range []float64{-1.25, 0, 99.75} {
		binary.BigEndian.PutUint64(data[0x10D+i*8:], math.Float64bits(value))
	}
	decoder, err := calibration.New(document, floatImage(t, data))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(curve)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.X, []float64{1000, 2500.5, 6000}) || !reflect.DeepEqual(decoded.Z, [][]float64{{-1.25, 0, 99.75}}) {
		t.Fatalf("decoded axis %v values %v", decoded.X, decoded.Z)
	}
}

func TestConvertRejectsFloatCount(t *testing.T) {
	source := syntheticSource(t, `
    /begin MOD_COMMON ""
      BYTE_ORDER MSB_FIRST
    /end MOD_COMMON
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%8.2" "-"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      NO_AXIS_PTS_X 1 FLOAT32_IEEE
      AXIS_PTS_X 2 FLOAT32_IEEE INDEX_INCR DIRECT
      FNC_VALUES 3 FLOAT32_IEEE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC Curve
      "a float axis-point count has no meaning"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      -100 100
      /begin AXIS_DESCR
        STD_AXIS
        M.RPM
        CM.IDENTICAL
        3
        0 8000
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertFloatRequiresByteOrder(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%8.2" "-"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 FLOAT32_IEEE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "float with no BYTE_ORDER anywhere"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      -1 1
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}
