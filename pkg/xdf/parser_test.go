package xdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSyntheticFixture(t *testing.T) {
	definition, err := Load(filepath.Join("testdata", "basic.xdf"))
	if err != nil {
		t.Fatal(err)
	}
	if definition.Version != "1.70" {
		t.Errorf("version = %q", definition.Version)
	}
	if len(definition.Categories) != 2 || len(definition.Tables) != 1 || len(definition.Flags) != 1 {
		t.Fatalf("counts = categories %d tables %d flags %d", len(definition.Categories), len(definition.Tables), len(definition.Flags))
	}
	region := definition.Header.Regions[0]
	if region.Start != 0 || region.Size != 0x400000 || region.End() != 0x3fffff {
		t.Errorf("region = %#x-%#x size %#x", region.Start, region.End(), region.Size)
	}
	if definition.Header.BaseOffset.Raw["future"] != "preserved" {
		t.Error("unknown BASEOFFSET attribute not preserved")
	}
	table := definition.Tables[0]
	if table.ID != "0x100" || table.Title != "Boost target" || len(table.Categories) != 1 || table.Categories[0] != 1 {
		t.Errorf("table = %#v", table)
	}
	if table.X.Formula != "X*10" || table.X.Units != "rpm" || table.X.IndexCount != 2 {
		t.Errorf("X axis = %#v", table.X)
	}
	if table.Z.Embedded.Address != 0x200 || table.Z.Embedded.RowCount != 2 || table.Z.Embedded.ColumnCount != 2 {
		t.Errorf("Z embedded = %#v", table.Z.Embedded)
	}
	if table.X.Embedded.Raw["custom"] != "kept" {
		t.Error("unknown embedded attribute not preserved")
	}
	if definition.Flags[0].Embedded.Address != 0x300 {
		t.Errorf("flag = %#v", definition.Flags[0])
	}
	if definition.Flags[0].Formula != "X" {
		t.Errorf("flag formula = %q", definition.Flags[0].Formula)
	}
}

func TestNegativeStrideIsPreserved(t *testing.T) {
	input := `<XDFFORMAT><XDFTABLE><title>T</title><XDFAXIS id="z"><EMBEDDEDDATA mmedaddress="0" mmedelementsizebits="8" mmedmajorstridebits="-32" mmedminorstridebits="-8"/></XDFAXIS></XDFTABLE></XDFFORMAT>`
	definition, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	embedded := definition.Tables[0].Z.Embedded
	if embedded.MajorStrideBits != -32 || embedded.MinorStrideBits != -8 {
		t.Fatalf("strides = %d, %d", embedded.MajorStrideBits, embedded.MinorStrideBits)
	}
}

func TestUnknownXMLIsIgnored(t *testing.T) {
	xml := `<XDFFORMAT version="9"><NEW/><XDFHEADER><UNKNOWN><DEEP/></UNKNOWN></XDFHEADER><XDFTABLE uniqueid="1"><title>T</title><MYSTERY/></XDFTABLE></XDFFORMAT>`
	definition, err := Parse(strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Tables) != 1 || definition.Tables[0].Title != "T" {
		t.Fatalf("definition = %#v", definition)
	}
}

func TestMalformedInputReturnsErrors(t *testing.T) {
	for _, input := range []string{
		`<notxdf/>`,
		`<XDFFORMAT><XDFHEADER><REGION startaddress="wat"/></XDFHEADER></XDFFORMAT>`,
		`<XDFFORMAT><XDFTABLE><title>T</title><XDFAXIS id="z"><EMBEDDEDDATA mmedaddress="-1"/></XDFAXIS></XDFTABLE></XDFFORMAT>`,
		`<XDFFORMAT>`,
	} {
		if _, err := Parse(strings.NewReader(input)); err == nil {
			t.Errorf("Parse(%q) unexpectedly succeeded", input)
		}
	}
}

func TestPrivateBMWXDF(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "private", "00000FEF81A001.xdf")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Skip("private BMW XDF fixture not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	definition, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Version != "1.70" {
		t.Errorf("version = %q", definition.Version)
	}
	if len(definition.Categories) != 73 {
		t.Errorf("categories = %d", len(definition.Categories))
	}
	if len(definition.Tables) != 962 {
		t.Errorf("tables = %d", len(definition.Tables))
	}
	if len(definition.Flags) != 7 {
		t.Errorf("flags = %d", len(definition.Flags))
	}
	if len(definition.Header.Regions) == 0 || definition.Header.Regions[0].Size != 0x400000 {
		t.Errorf("regions = %#v", definition.Header.Regions)
	}
}
