package xdf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"

	"github.com/freebeamer/core/pkg/types"
)

func Load(path string) (*types.XDFDefinition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open XDF %q: %w", path, err)
	}
	defer file.Close()
	definition, err := Parse(file)
	if err != nil {
		return nil, fmt.Errorf("load XDF %q: %w", path, err)
	}
	return definition, nil
}

func Parse(reader io.Reader) (*types.XDFDefinition, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read XDF: %w", err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var raw types.XDFXMLDocument
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse XDF: %w", err)
	}
	if raw.XMLName.Local != "XDFFORMAT" {
		return nil, fmt.Errorf("parse XDF: root element is %q, want XDFFORMAT", raw.XMLName.Local)
	}
	definition := &types.XDFDefinition{
		Version: raw.Version,
		Header: types.XDFHeader{
			Title:       raw.Header.Title,
			Description: raw.Header.Description,
			Author:      raw.Header.Author,
		},
		RawXML: append([]byte(nil), data...),
	}
	if definition.Header.BaseOffset, err = normalizeBaseOffset(raw.Header.BaseOffset); err != nil {
		return nil, err
	}
	if definition.Header.Defaults, err = normalizeDefaults(raw.Header.Defaults); err != nil {
		return nil, err
	}
	for index, region := range raw.Header.Regions {
		value, parseErr := normalizeRegion(region)
		if parseErr != nil {
			return nil, fmt.Errorf("XDF region %d: %w", index, parseErr)
		}
		definition.Header.Regions = append(definition.Header.Regions, value)
	}
	for index, category := range raw.Header.Categories {
		value, parseErr := normalizeCategory(category)
		if parseErr != nil {
			return nil, fmt.Errorf("XDF category %d: %w", index, parseErr)
		}
		definition.Categories = append(definition.Categories, value)
	}
	for index, table := range raw.Tables {
		value, parseErr := normalizeTable(table)
		if parseErr != nil {
			return nil, fmt.Errorf("XDF table %d (%q): %w", index, table.Title, parseErr)
		}
		definition.Tables = append(definition.Tables, value)
	}
	for index, flag := range raw.Flags {
		value, parseErr := normalizeFlag(flag)
		if parseErr != nil {
			return nil, fmt.Errorf("XDF flag %d (%q): %w", index, flag.Title, parseErr)
		}
		definition.Flags = append(definition.Flags, value)
	}
	for index, constant := range raw.Constants {
		value, parseErr := normalizeConstant(constant)
		if parseErr != nil {
			return nil, fmt.Errorf("XDF constant %d (%q): %w", index, constant.Title, parseErr)
		}
		definition.Constants = append(definition.Constants, value)
	}
	return definition, nil
}
