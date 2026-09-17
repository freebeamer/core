package xdf

import (
	"strings"

	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeFlag(raw types.XDFXMLFlag) (types.XDFFlag, error) {
	values := utils.Attrs(raw.Attrs)
	memberships, err := normalizeCategoryMemberships(raw.Categories)
	if err != nil {
		return types.XDFFlag{}, err
	}
	embedded, err := normalizeEmbedded(raw.Embedded)
	if err != nil {
		return types.XDFFlag{}, err
	}
	mask, err := normalizeMask(raw.Mask)
	if err != nil {
		return types.XDFFlag{}, err
	}
	return types.XDFFlag{
		ID:          values["uniqueid"],
		Title:       strings.TrimSpace(raw.Title),
		Description: strings.TrimSpace(raw.Description),
		Categories:  memberships,
		Embedded:    embedded,
		Formula:     utils.Attrs(raw.Math.Attrs)["equation"],
		Mask:        mask,
		Raw:         values,
	}, nil
}

// normalizeMask parses an XDFFLAG's <mask> element text, if present. Two
// independent open-source XDF implementations (OpenEEC's SADXdf.cs and the
// TunerPro-XDF-BIN-Universal-Exporter project) agree that <mask> is a child
// element holding the bitmask applied to the addressed byte; neither is an
// authoritative TunerPro specification, so an absent <mask> is left nil
// rather than assumed to mean bit 0.
func normalizeMask(text string) (*uint64, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	mask, err := utils.Integer(map[string]string{"mask": text}, "mask", 64)
	if err != nil {
		return nil, err
	}
	return &mask, nil
}
