package calibration

import (
	"errors"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

// bitFlag builds a single-bit boolean parameter at address 0, matching what
// the XDF converter produces for a flag whose <mask> resolves to one bit.
func bitFlag(id string, bitPosition int) types.MapParameter {
	address := uint64(0)
	position := bitPosition
	return types.MapParameter{
		ID:   id,
		Kind: "flag",
		Name: id,
		Layout: types.DataLayout{
			Address:     &address,
			DataType:    "uint8",
			ByteOrder:   "not-applicable",
			Dimensions:  []int{1},
			Order:       "not-applicable",
			BitPosition: &position,
		},
		Conversion: types.IdentityConversion(),
	}
}

func definitionWithParameters(parameters ...types.MapParameter) *types.MapDefinition {
	return &types.MapDefinition{
		Format: "freehorse.mapdef", FormatVersion: "1.0.0", ID: "test", Name: "test",
		Parameters: parameters,
	}
}

func TestDecodeExtractsTheAddressedBit(t *testing.T) {
	// 0x06 = 0b00000110: bit 0 clear, bits 1 and 2 set.
	def := definitionWithParameters(bitFlag("bit0", 0), bitFlag("bit1", 1), bitFlag("bit2", 2))
	decoder, err := New(def, imageFor(t, []byte{0x06}))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"bit0": 0, "bit1": 1, "bit2": 1}
	for _, parameter := range def.Parameters {
		decoded, err := decoder.DecodeParameter(parameter)
		if err != nil {
			t.Fatalf("%s: %v", parameter.ID, err)
		}
		if got := decoded.Z[0][0]; got != want[parameter.ID] {
			t.Errorf("%s decoded = %v, want %v", parameter.ID, got, want[parameter.ID])
		}
	}
}

func TestSetCellWritesOnlyItsOwnBitLeavingSiblingFlagsIntact(t *testing.T) {
	bit0, bit2 := bitFlag("bit0", 0), bitFlag("bit2", 2)
	def := definitionWithParameters(bit0, bit2)
	image := imageFor(t, []byte{0x00})
	decoder, err := New(def, image)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := decoder.SetIndex(bit0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if got := image.Bytes()[0]; got != 0x01 {
		t.Fatalf("after setting bit 0, byte = %#x, want 0x01", got)
	}
	if decoded, err := decoder.DecodeParameter(bit2); err != nil || decoded.Z[0][0] != 0 {
		t.Fatalf("sibling bit 2 disturbed: decoded=%v err=%v", decoded, err)
	}

	if _, err := decoder.SetIndex(bit2, 0, 1); err != nil {
		t.Fatal(err)
	}
	if got := image.Bytes()[0]; got != 0x05 {
		t.Fatalf("after setting bit 2, byte = %#x, want 0x05", got)
	}
	if decoded, err := decoder.DecodeParameter(bit0); err != nil || decoded.Z[0][0] != 1 {
		t.Fatalf("sibling bit 0 disturbed: decoded=%v err=%v", decoded, err)
	}

	result, err := decoder.SetIndex(bit0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := image.Bytes()[0]; got != 0x04 {
		t.Fatalf("after clearing bit 0, byte = %#x, want 0x04", got)
	}
	if result.Previous[0] != 0x05 {
		t.Fatalf("Previous = %#x, want the pre-clear byte 0x05", result.Previous[0])
	}
}

func TestSetCellRejectsNonBooleanRawForABitPosition(t *testing.T) {
	flag := bitFlag("bit0", 0)
	def := definitionWithParameters(flag)
	image := imageFor(t, []byte{0x00})
	decoder, err := New(def, image)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.SetIndex(flag, 0, 2); !errors.Is(err, ErrRawOutOfRange) {
		t.Fatalf("error = %v, want ErrRawOutOfRange", err)
	}
	if len(image.Diff()) != 0 {
		t.Fatal("rejected write mutated the image")
	}
}

func TestBitPositionExceedingStorageWidthErrorsWithoutPanicking(t *testing.T) {
	flag := bitFlag("bit8", 8)
	def := definitionWithParameters(flag)
	image := imageFor(t, []byte{0x00})
	decoder, err := New(def, image)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.DecodeParameter(flag); !errors.Is(err, ErrInvalidBitPosition) {
		t.Fatalf("decode error = %v, want ErrInvalidBitPosition", err)
	}
	if _, err := decoder.SetIndex(flag, 0, 1); !errors.Is(err, ErrInvalidBitPosition) {
		t.Fatalf("set error = %v, want ErrInvalidBitPosition", err)
	}
	if len(image.Diff()) != 0 {
		t.Fatal("rejected write mutated the image")
	}
}
