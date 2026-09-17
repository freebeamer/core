package types_test

import (
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func TestXDFRegionEnd(t *testing.T) {
	tests := []struct {
		name   string
		region types.XDFRegion
		want   uint64
	}{
		{
			name:   "non-empty region",
			region: types.XDFRegion{Start: 0x100, Size: 0x20},
			want:   0x11f,
		},
		{
			name:   "empty region",
			region: types.XDFRegion{Start: 0x100},
			want:   0x100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.region.End(); got != test.want {
				t.Fatalf("End() = %#x, want %#x", got, test.want)
			}
		})
	}
}

func TestXDFDefinitionRegionsFit(t *testing.T) {
	tests := []struct {
		name       string
		definition types.XDFDefinition
		imageSize  uint64
		want       bool
	}{
		{
			name:      "no declared regions",
			imageSize: 0x200,
			want:      false,
		},
		{
			name: "all regions fit",
			definition: types.XDFDefinition{Header: types.XDFHeader{Regions: []types.XDFRegion{
				{Start: 0x00, Size: 0x20},
				{Start: 0x80, Size: 0x80},
			}}},
			imageSize: 0x100,
			want:      true,
		},
		{
			name: "region extends past image",
			definition: types.XDFDefinition{Header: types.XDFHeader{Regions: []types.XDFRegion{
				{Start: 0xf0, Size: 0x20},
			}}},
			imageSize: 0x100,
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.definition.RegionsFit(test.imageSize); got != test.want {
				t.Fatalf("RegionsFit(%#x) = %t, want %t", test.imageSize, got, test.want)
			}
		})
	}
}

func TestXDFDefinitionCategoryNames(t *testing.T) {
	definition := types.XDFDefinition{Categories: []types.XDFCategory{
		{Index: 1, Name: "Fuel"},
		{Index: 2, Name: "Ignition"},
	}}

	got := definition.CategoryNames([]int{2, 9, 1})
	want := []string{"0x9", "Fuel", "Ignition"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CategoryNames() = %v, want %v", got, want)
	}
}
