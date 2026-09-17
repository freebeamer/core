package types_test

import (
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func TestIdentityConversion(t *testing.T) {
	got := types.IdentityConversion()
	if got.Language != "freehorse-expression-v1" {
		t.Fatalf("Language = %q", got.Language)
	}
	if got.Expression != "X" {
		t.Fatalf("Expression = %q", got.Expression)
	}
}

func TestMapDefinitionMemoryFits(t *testing.T) {
	definition := types.MapDefinition{Memory: types.MapMemory{Segments: []types.MemorySegment{
		{Start: 0x10, Size: 0x20},
	}}}
	if !definition.MemoryFits(0x30) {
		t.Fatal("expected declared memory to fit")
	}
	if definition.MemoryFits(0x2f) {
		t.Fatal("expected declared memory to exceed image")
	}
}

func TestMapDefinitionCategoryNames(t *testing.T) {
	definition := types.MapDefinition{Categories: []types.MapCategory{
		{ID: "fuel", Name: "Fuel"},
		{ID: "boost", Name: "Boost"},
	}}
	got := definition.CategoryNames([]string{"fuel", "unknown", "boost"})
	want := []string{"Boost", "Fuel", "unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CategoryNames() = %v, want %v", got, want)
	}
}
