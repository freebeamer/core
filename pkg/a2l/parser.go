package a2l

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/freebeamer/core/pkg/types"
)

// Load reads and parses an A2L file from path.
func Load(path string) (*types.A2LDefinition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open A2L %q: %w", path, err)
	}
	defer file.Close()
	definition, err := Parse(file)
	if err != nil {
		return nil, fmt.Errorf("load A2L %q: %w", path, err)
	}
	return definition, nil
}

// Parse reads one A2L document. See docs/a2l-v0-plan.md for exactly which
// constructs are interpreted; everything else is skipped as an opaque,
// balanced /begin.../end block and remains recoverable only from the raw
// source text this returns on A2LDefinition.RawText.
func Parse(reader io.Reader) (*types.A2LDefinition, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read A2L: %w", err)
	}
	tokens, err := tokenize(data)
	if err != nil {
		return nil, fmt.Errorf("parse A2L: %w", err)
	}
	p := &parser{tokens: tokens}
	definition := &types.A2LDefinition{RawText: append([]byte(nil), data...)}
	if err := p.parseDocument(definition); err != nil {
		return nil, fmt.Errorf("parse A2L: %w", err)
	}
	return definition, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *parser) expectIdent() (string, error) {
	t := p.advance()
	if t.kind != tokIdent {
		return "", fmt.Errorf("line %d: expected an identifier, got %s", t.line, t.describe())
	}
	return t.text, nil
}

func (p *parser) expectIdentKeyword(word string) error {
	t := p.advance()
	if t.kind != tokIdent || t.text != word {
		return fmt.Errorf("line %d: expected %q, got %s", t.line, word, t.describe())
	}
	return nil
}

func (p *parser) expectBeginKeyword(word string) error {
	t := p.advance()
	if t.kind != tokBegin {
		return fmt.Errorf("line %d: expected /begin %s, got %s", t.line, word, t.describe())
	}
	return p.expectIdentKeyword(word)
}

func (p *parser) expectEndKeyword(word string) error {
	t := p.advance()
	if t.kind != tokEnd {
		return fmt.Errorf("line %d: expected /end %s, got %s", t.line, word, t.describe())
	}
	return p.expectIdentKeyword(word)
}

func (p *parser) expectString() (string, error) {
	t := p.advance()
	if t.kind != tokString {
		return "", fmt.Errorf("line %d: expected a string, got %s", t.line, t.describe())
	}
	return t.text, nil
}

func (p *parser) expectFloat() (float64, error) {
	t := p.advance()
	if t.kind != tokNumber {
		return 0, fmt.Errorf("line %d: expected a number, got %s", t.line, t.describe())
	}
	value, err := strconv.ParseFloat(t.text, 64)
	if err != nil {
		return 0, fmt.Errorf("line %d: invalid number %q: %w", t.line, t.text, err)
	}
	return value, nil
}

func (p *parser) expectInt() (int, error) {
	t := p.advance()
	if t.kind != tokNumber {
		return 0, fmt.Errorf("line %d: expected a number, got %s", t.line, t.describe())
	}
	value, err := strconv.ParseInt(t.text, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("line %d: invalid integer %q: %w", t.line, t.text, err)
	}
	return int(value), nil
}

func (p *parser) expectAddress() (uint64, error) {
	t := p.advance()
	if t.kind != tokNumber {
		return 0, fmt.Errorf("line %d: expected an address, got %s", t.line, t.describe())
	}
	value, err := strconv.ParseUint(t.text, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("line %d: invalid address %q: %w", t.line, t.text, err)
	}
	return value, nil
}

// skipBlock consumes tokens until the /end matching the /begin already
// consumed by the caller, tracking nested /begin/.../end pairs so a skipped
// block's own children (of any keyword, including ones this parser has
// never heard of) are consumed correctly without being interpreted.
func (p *parser) skipBlock() error {
	depth := 1
	for depth > 0 {
		t := p.advance()
		switch t.kind {
		case tokBegin:
			depth++
		case tokEnd:
			depth--
			if depth == 0 {
				// Consume the closing keyword identifier that follows this
				// /end (e.g. the "HEADER" in "/end HEADER"); nested /end
				// keywords are consumed naturally by the next loop
				// iteration since they don't affect depth.
				p.advance()
			}
		case tokEOF:
			return errors.New("unexpected end of file while skipping a block")
		}
	}
	return nil
}

func (p *parser) parseDocument(definition *types.A2LDefinition) error {
	if p.peek().kind == tokIdent && p.peek().text == "ASAP2_VERSION" {
		p.advance()
		if _, err := p.expectInt(); err != nil {
			return err
		}
		if _, err := p.expectInt(); err != nil {
			return err
		}
	}
	if err := p.expectBeginKeyword("PROJECT"); err != nil {
		return err
	}
	name, err := p.expectIdent()
	if err != nil {
		return err
	}
	description, err := p.expectString()
	if err != nil {
		return err
	}
	definition.Project.Name = name
	definition.Project.Description = description

	foundModule := false
	for {
		t := p.peek()
		if t.kind == tokEnd {
			if err := p.expectEndKeyword("PROJECT"); err != nil {
				return err
			}
			if !foundModule {
				return errors.New("no MODULE found in PROJECT")
			}
			return nil
		}
		if t.kind != tokBegin {
			return fmt.Errorf("line %d: expected /begin or /end inside PROJECT, got %s", t.line, t.describe())
		}
		p.advance()
		keyword, err := p.expectIdent()
		if err != nil {
			return err
		}
		if keyword == "MODULE" {
			if err := p.parseModule(&definition.Project.Module); err != nil {
				return err
			}
			foundModule = true
			continue
		}
		if err := p.skipBlock(); err != nil {
			return fmt.Errorf("%s: %w", keyword, err)
		}
	}
}

func (p *parser) parseModule(module *types.A2LModule) error {
	name, err := p.expectIdent()
	if err != nil {
		return err
	}
	description, err := p.expectString()
	if err != nil {
		return err
	}
	module.Name = name
	module.Description = description

	for {
		t := p.peek()
		if t.kind == tokEnd {
			return p.expectEndKeyword("MODULE")
		}
		if t.kind != tokBegin {
			return fmt.Errorf("line %d: expected /begin or /end inside MODULE, got %s", t.line, t.describe())
		}
		p.advance()
		keyword, err := p.expectIdent()
		if err != nil {
			return err
		}
		switch keyword {
		case "COMPU_METHOD":
			compuMethod, err := p.parseCompuMethod()
			if err != nil {
				return fmt.Errorf("COMPU_METHOD: %w", err)
			}
			module.CompuMethods = append(module.CompuMethods, compuMethod)
		case "COMPU_VTAB":
			compuVtab, err := p.parseCompuVtab()
			if err != nil {
				return fmt.Errorf("COMPU_VTAB: %w", err)
			}
			module.CompuVtabs = append(module.CompuVtabs, compuVtab)
		case "COMPU_VTAB_RANGE":
			compuVtabRange, err := p.parseCompuVtabRange()
			if err != nil {
				return fmt.Errorf("COMPU_VTAB_RANGE: %w", err)
			}
			module.CompuVtabRanges = append(module.CompuVtabRanges, compuVtabRange)
		case "COMPU_TAB":
			compuTab, err := p.parseCompuTab()
			if err != nil {
				return fmt.Errorf("COMPU_TAB: %w", err)
			}
			module.CompuTabs = append(module.CompuTabs, compuTab)
		case "RECORD_LAYOUT":
			recordLayout, err := p.parseRecordLayout()
			if err != nil {
				return fmt.Errorf("RECORD_LAYOUT: %w", err)
			}
			module.RecordLayouts = append(module.RecordLayouts, recordLayout)
		case "CHARACTERISTIC":
			characteristic, err := p.parseCharacteristic()
			if err != nil {
				return fmt.Errorf("CHARACTERISTIC: %w", err)
			}
			module.Characteristics = append(module.Characteristics, characteristic)
		case "MOD_COMMON":
			byteOrder, err := p.parseModCommon()
			if err != nil {
				return fmt.Errorf("MOD_COMMON: %w", err)
			}
			module.ByteOrder = byteOrder
		case "AXIS_PTS":
			axisPts, err := p.parseAxisPts()
			if err != nil {
				return fmt.Errorf("AXIS_PTS: %w", err)
			}
			module.AxisPts = append(module.AxisPts, axisPts)
		case "GROUP":
			group, err := p.parseGroup()
			if err != nil {
				return fmt.Errorf("GROUP: %w", err)
			}
			module.Groups = append(module.Groups, group)
		default:
			if err := p.skipBlock(); err != nil {
				return fmt.Errorf("%s: %w", keyword, err)
			}
		}
	}
}

// parseModCommon reads MOD_COMMON's mandatory description followed by any
// number of its optional keywords in any order, returning BYTE_ORDER's
// value (empty if absent). Only the keywords real MOD_COMMON blocks are
// evidenced to use are recognized (see docs/a2l-v0-plan.md); anything else
// makes the block fail to parse rather than being guessed at.
func (p *parser) parseModCommon() (string, error) {
	if _, err := p.expectString(); err != nil {
		return "", err
	}
	byteOrder := ""
	for p.peek().kind == tokIdent {
		keyword, err := p.expectIdent()
		if err != nil {
			return byteOrder, err
		}
		switch keyword {
		case "BYTE_ORDER":
			if byteOrder, err = p.expectIdent(); err != nil {
				return byteOrder, err
			}
		case "DEPOSIT":
			if _, err := p.expectIdent(); err != nil {
				return byteOrder, err
			}
		case "ALIGNMENT_BYTE", "ALIGNMENT_WORD", "ALIGNMENT_LONG", "ALIGNMENT_FLOAT32_IEEE", "ALIGNMENT_FLOAT64_IEEE":
			if _, err := p.expectInt(); err != nil {
				return byteOrder, err
			}
		default:
			return byteOrder, fmt.Errorf("unsupported optional keyword %q (this slice only recognizes BYTE_ORDER, DEPOSIT, ALIGNMENT_*)", keyword)
		}
	}
	if err := p.expectEndKeyword("MOD_COMMON"); err != nil {
		return byteOrder, err
	}
	return byteOrder, nil
}

func (p *parser) parseCompuMethod() (types.A2LCompuMethod, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LCompuMethod{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LCompuMethod{}, err
	}
	conversionType, err := p.expectIdent()
	if err != nil {
		return types.A2LCompuMethod{}, err
	}
	format, err := p.expectString()
	if err != nil {
		return types.A2LCompuMethod{}, err
	}
	unit, err := p.expectString()
	if err != nil {
		return types.A2LCompuMethod{}, err
	}
	result := types.A2LCompuMethod{Name: name, Description: description, ConversionType: conversionType, Format: format, Unit: unit}
	for p.peek().kind == tokIdent || p.peek().kind == tokBegin {
		if p.peek().kind == tokBegin {
			p.advance()
			if err := p.expectIdentKeyword("FORMULA"); err != nil {
				return result, err
			}
			formula, err := p.expectString()
			if err != nil {
				return result, err
			}
			result.Formula = formula
			if p.peek().kind == tokIdent {
				if err := p.expectIdentKeyword("FORMULA_INV"); err != nil {
					return result, err
				}
				formulaInv, err := p.expectString()
				if err != nil {
					return result, err
				}
				result.FormulaInv = formulaInv
			}
			if err := p.expectEndKeyword("FORMULA"); err != nil {
				return result, err
			}
			continue
		}
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		switch keyword {
		case "COEFFS_LINEAR":
			if result.CoeffsLinearA, err = p.expectFloat(); err != nil {
				return result, err
			}
			if result.CoeffsLinearB, err = p.expectFloat(); err != nil {
				return result, err
			}
			result.HasCoeffsLinear = true
		case "COEFFS":
			if result.CoeffsA, err = p.expectFloat(); err != nil {
				return result, err
			}
			if result.CoeffsB, err = p.expectFloat(); err != nil {
				return result, err
			}
			if result.CoeffsC, err = p.expectFloat(); err != nil {
				return result, err
			}
			if result.CoeffsD, err = p.expectFloat(); err != nil {
				return result, err
			}
			if result.CoeffsE, err = p.expectFloat(); err != nil {
				return result, err
			}
			if result.CoeffsF, err = p.expectFloat(); err != nil {
				return result, err
			}
			result.HasCoeffs = true
		case "COMPU_TAB_REF":
			if result.CompuTabRef, err = p.expectIdent(); err != nil {
				return result, err
			}
		default:
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes COEFFS_LINEAR, COEFFS, COMPU_TAB_REF, FORMULA)", name, keyword)
		}
	}
	if err := p.expectEndKeyword("COMPU_METHOD"); err != nil {
		return result, err
	}
	return result, nil
}

// parseCompuVtab reads a COMPU_VTAB block: its name/description, the
// literal keyword TAB_VERB followed by the pair count (a repeated
// positional marker, not a reference to the COMPU_METHOD's own
// ConversionType), then exactly that many <Value> "<Text>" pairs, then an
// optional DEFAULT_VALUE.
func (p *parser) parseCompuVtab() (types.A2LCompuVtab, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LCompuVtab{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LCompuVtab{}, err
	}
	if err := p.expectIdentKeyword("TAB_VERB"); err != nil {
		return types.A2LCompuVtab{}, fmt.Errorf("%s: %w (this slice only supports the TAB_VERB shape of COMPU_VTAB)", name, err)
	}
	count, err := p.expectInt()
	if err != nil {
		return types.A2LCompuVtab{}, err
	}
	result := types.A2LCompuVtab{Name: name, Description: description}
	for i := 0; i < count; i++ {
		entry := types.A2LCompuVtabEntry{}
		if entry.Value, err = p.expectInt(); err != nil {
			return result, err
		}
		if entry.Text, err = p.expectString(); err != nil {
			return result, err
		}
		result.Entries = append(result.Entries, entry)
	}
	for p.peek().kind == tokIdent {
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		if keyword != "DEFAULT_VALUE" {
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes DEFAULT_VALUE)", name, keyword)
		}
		if result.DefaultValue, err = p.expectString(); err != nil {
			return result, err
		}
		result.HasDefaultValue = true
	}
	if err := p.expectEndKeyword("COMPU_VTAB"); err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

// parseCompuVtabRange reads a COMPU_VTAB_RANGE block: its name/description,
// the triple count (unlike COMPU_VTAB, there is no leading TAB_VERB literal
// keyword here — real evidence and pyA2L's own class definition both show
// NumberValueTriples immediately following LongIdentifier), then exactly
// that many <LowerValue> <UpperValue> "<Text>" triples, then an optional
// DEFAULT_VALUE. Converting a Conversion that resolves to this block is
// rejected elsewhere (ErrCompuVtabRangeUnsupported) — see docs/a2l-v0-plan.md.
func (p *parser) parseCompuVtabRange() (types.A2LCompuVtabRange, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LCompuVtabRange{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LCompuVtabRange{}, err
	}
	count, err := p.expectInt()
	if err != nil {
		return types.A2LCompuVtabRange{}, err
	}
	result := types.A2LCompuVtabRange{Name: name, Description: description}
	for i := 0; i < count; i++ {
		entry := types.A2LCompuVtabRangeEntry{}
		if entry.LowerValue, err = p.expectInt(); err != nil {
			return result, err
		}
		if entry.UpperValue, err = p.expectInt(); err != nil {
			return result, err
		}
		if entry.Text, err = p.expectString(); err != nil {
			return result, err
		}
		result.Entries = append(result.Entries, entry)
	}
	for p.peek().kind == tokIdent {
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		if keyword != "DEFAULT_VALUE" {
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes DEFAULT_VALUE)", name, keyword)
		}
		if result.DefaultValue, err = p.expectString(); err != nil {
			return result, err
		}
		result.HasDefaultValue = true
	}
	if err := p.expectEndKeyword("COMPU_VTAB_RANGE"); err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

// parseCompuTab reads a COMPU_TAB block: its name/description, the
// ConversionType (TAB_INTP or TAB_NOINTP — unlike COMPU_VTAB, this is a
// real variable field, not a fixed literal keyword), the pair count, then
// exactly that many <InVal> <OutVal> numeric pairs, then an optional
// DEFAULT_VALUE_NUMERIC. A TAB_INTP/TAB_NOINTP COMPU_METHOD references
// this via COMPU_TAB_REF — see docs/a2l-v0-plan.md's COMPU_TAB evidence.
func (p *parser) parseCompuTab() (types.A2LCompuTab, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LCompuTab{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LCompuTab{}, err
	}
	conversionType, err := p.expectIdent()
	if err != nil {
		return types.A2LCompuTab{}, err
	}
	var interpolated bool
	switch conversionType {
	case "TAB_INTP":
		interpolated = true
	case "TAB_NOINTP":
		interpolated = false
	default:
		return types.A2LCompuTab{}, fmt.Errorf("%s: unsupported COMPU_TAB ConversionType %q (this slice only recognizes TAB_INTP, TAB_NOINTP)", name, conversionType)
	}
	count, err := p.expectInt()
	if err != nil {
		return types.A2LCompuTab{}, err
	}
	result := types.A2LCompuTab{Name: name, Description: description, Interpolated: interpolated}
	for i := 0; i < count; i++ {
		entry := types.A2LCompuTabEntry{}
		if entry.InVal, err = p.expectFloat(); err != nil {
			return result, err
		}
		if entry.OutVal, err = p.expectFloat(); err != nil {
			return result, err
		}
		result.Entries = append(result.Entries, entry)
	}
	for p.peek().kind == tokIdent {
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		if keyword != "DEFAULT_VALUE_NUMERIC" {
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes DEFAULT_VALUE_NUMERIC)", name, keyword)
		}
		if result.DefaultValue, err = p.expectFloat(); err != nil {
			return result, err
		}
		result.HasDefaultValue = true
	}
	if err := p.expectEndKeyword("COMPU_TAB"); err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

// parseRecordLayout reads a RECORD_LAYOUT's name followed by any number of
// its FNC_VALUES/AXIS_PTS_X/AXIS_PTS_Y/NO_AXIS_PTS_X/NO_AXIS_PTS_Y entries,
// in any order. Any other keyword makes the block fail to parse rather than
// being guessed at; deciding which combination of entries is actually valid
// for a given CHARACTERISTIC type is the converter's job, not the parser's.
func (p *parser) parseRecordLayout() (types.A2LRecordLayout, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LRecordLayout{}, err
	}
	result := types.A2LRecordLayout{Name: name}
	for p.peek().kind == tokIdent {
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		entry := types.A2LRecordLayoutEntry{Role: keyword}
		switch keyword {
		case "FNC_VALUES", "AXIS_PTS_X", "AXIS_PTS_Y":
			if entry.Position, err = p.expectInt(); err != nil {
				return result, err
			}
			if entry.DataType, err = p.expectIdent(); err != nil {
				return result, err
			}
			if entry.IndexMode, err = p.expectIdent(); err != nil {
				return result, err
			}
			if entry.AddrType, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "AXIS_RESCALE_X":
			if entry.Position, err = p.expectInt(); err != nil {
				return result, err
			}
			if entry.DataType, err = p.expectIdent(); err != nil {
				return result, err
			}
			if entry.RescalePairs, err = p.expectInt(); err != nil {
				return result, err
			}
			if entry.IndexMode, err = p.expectIdent(); err != nil {
				return result, err
			}
			if entry.AddrType, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "NO_AXIS_PTS_X", "NO_AXIS_PTS_Y", "NO_RESCALE_X", "RESERVED":
			if entry.Position, err = p.expectInt(); err != nil {
				return result, err
			}
			if entry.DataType, err = p.expectIdent(); err != nil {
				return result, err
			}
		default:
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes FNC_VALUES, AXIS_PTS_X, AXIS_PTS_Y, NO_AXIS_PTS_X, NO_AXIS_PTS_Y, AXIS_RESCALE_X, NO_RESCALE_X, RESERVED)", name, keyword)
		}
		result.Entries = append(result.Entries, entry)
	}
	if err := p.expectEndKeyword("RECORD_LAYOUT"); err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	if len(result.Entries) == 0 {
		return result, fmt.Errorf("%s: RECORD_LAYOUT has no recognized entries", name)
	}
	return result, nil
}

func (p *parser) parseCharacteristic() (types.A2LCharacteristic, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	characteristicType, err := p.expectIdent()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	address, err := p.expectAddress()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	deposit, err := p.expectIdent()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	maxDiff, err := p.expectFloat()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	conversion, err := p.expectIdent()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	lowerLimit, err := p.expectFloat()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	upperLimit, err := p.expectFloat()
	if err != nil {
		return types.A2LCharacteristic{}, err
	}
	result := types.A2LCharacteristic{
		Name: name, Description: description, Type: characteristicType,
		Address: address, Deposit: deposit, MaxDiff: maxDiff,
		Conversion: conversion, LowerLimit: lowerLimit, UpperLimit: upperLimit,
	}
	for {
		t := p.peek()
		if t.kind == tokEnd {
			break
		}
		if t.kind == tokBegin {
			p.advance()
			keyword, err := p.expectIdent()
			if err != nil {
				return result, err
			}
			if keyword != "AXIS_DESCR" {
				return result, fmt.Errorf("%s: unsupported nested block %q (this slice only recognizes AXIS_DESCR)", name, keyword)
			}
			axisDescr, err := p.parseAxisDescr()
			if err != nil {
				return result, fmt.Errorf("%s: AXIS_DESCR: %w", name, err)
			}
			result.AxisDescrs = append(result.AxisDescrs, axisDescr)
			continue
		}
		if t.kind != tokIdent {
			return result, fmt.Errorf("line %d: expected an optional keyword, AXIS_DESCR, or /end CHARACTERISTIC, got %s", t.line, t.describe())
		}
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		switch keyword {
		case "FORMAT":
			if result.Format, err = p.expectString(); err != nil {
				return result, err
			}
		case "DISPLAY_IDENTIFIER":
			if result.DisplayIdentifier, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "EXTENDED_LIMITS":
			limits := &types.A2LExtendedLimits{}
			if limits.Low, err = p.expectFloat(); err != nil {
				return result, err
			}
			if limits.High, err = p.expectFloat(); err != nil {
				return result, err
			}
			result.ExtendedLimits = limits
		case "BYTE_ORDER":
			if result.ByteOrder, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "MATRIX_DIM":
			dim := &types.A2LMatrixDim{}
			if dim.X, err = p.expectInt(); err != nil {
				return result, err
			}
			if dim.Y, err = p.expectInt(); err != nil {
				return result, err
			}
			if dim.Z, err = p.expectInt(); err != nil {
				return result, err
			}
			result.MatrixDim = dim
		default:
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes FORMAT, DISPLAY_IDENTIFIER, EXTENDED_LIMITS, BYTE_ORDER, MATRIX_DIM)", name, keyword)
		}
	}
	if err := p.expectEndKeyword("CHARACTERISTIC"); err != nil {
		return result, err
	}
	return result, nil
}

// parseAxisDescr reads one AXIS_DESCR block: its five mandatory positional
// fields, then any number of optional keywords (and, for FIX_AXIS_PAR_LIST,
// one nested block) in any order.
func (p *parser) parseAxisDescr() (types.A2LAxisDescr, error) {
	attribute, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisDescr{}, err
	}
	inputQuantity, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisDescr{}, err
	}
	conversion, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisDescr{}, err
	}
	maxAxisPoints, err := p.expectInt()
	if err != nil {
		return types.A2LAxisDescr{}, err
	}
	lowerLimit, err := p.expectFloat()
	if err != nil {
		return types.A2LAxisDescr{}, err
	}
	upperLimit, err := p.expectFloat()
	if err != nil {
		return types.A2LAxisDescr{}, err
	}
	result := types.A2LAxisDescr{
		Attribute: attribute, InputQuantity: inputQuantity, Conversion: conversion,
		MaxAxisPoints: maxAxisPoints, LowerLimit: lowerLimit, UpperLimit: upperLimit,
	}
	for p.peek().kind == tokIdent || p.peek().kind == tokBegin {
		if p.peek().kind == tokBegin {
			p.advance()
			if err := p.expectIdentKeyword("FIX_AXIS_PAR_LIST"); err != nil {
				return result, err
			}
			for p.peek().kind == tokNumber {
				value, err := p.expectInt()
				if err != nil {
					return result, err
				}
				result.FixAxisParList = append(result.FixAxisParList, value)
			}
			if err := p.expectEndKeyword("FIX_AXIS_PAR_LIST"); err != nil {
				return result, err
			}
			continue
		}
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		switch keyword {
		case "MONOTONY":
			if result.Monotony, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "AXIS_PTS_REF":
			if result.AxisPtsRef, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "CURVE_AXIS_REF":
			if result.CurveAxisRef, err = p.expectIdent(); err != nil {
				return result, err
			}
		case "FIX_AXIS_PAR_DIST":
			dist := &types.A2LFixAxisParDist{}
			if dist.Offset, err = p.expectInt(); err != nil {
				return result, err
			}
			if dist.Distance, err = p.expectInt(); err != nil {
				return result, err
			}
			if dist.NumberPts, err = p.expectInt(); err != nil {
				return result, err
			}
			result.FixAxisParDist = dist
		default:
			return result, fmt.Errorf("unsupported optional keyword %q (this slice only recognizes MONOTONY, AXIS_PTS_REF, CURVE_AXIS_REF, FIX_AXIS_PAR_DIST, FIX_AXIS_PAR_LIST)", keyword)
		}
	}
	if err := p.expectEndKeyword("AXIS_DESCR"); err != nil {
		return result, err
	}
	return result, nil
}

// parseAxisPts reads an AXIS_PTS block: its 9 mandatory positional fields
// (the same shape as CHARACTERISTIC's, minus Type, plus InputQuantity and
// MaxAxisPoints), then any number of optional keywords in any order.
func (p *parser) parseAxisPts() (types.A2LAxisPts, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	address, err := p.expectAddress()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	inputQuantity, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	deposit, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	maxDiff, err := p.expectFloat()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	conversion, err := p.expectIdent()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	maxAxisPoints, err := p.expectInt()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	lowerLimit, err := p.expectFloat()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	upperLimit, err := p.expectFloat()
	if err != nil {
		return types.A2LAxisPts{}, err
	}
	result := types.A2LAxisPts{
		Name: name, Description: description, Address: address, InputQuantity: inputQuantity,
		Deposit: deposit, MaxDiff: maxDiff, Conversion: conversion,
		MaxAxisPoints: maxAxisPoints, LowerLimit: lowerLimit, UpperLimit: upperLimit,
	}
	for p.peek().kind == tokIdent {
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		switch keyword {
		case "DISPLAY_IDENTIFIER":
			if result.DisplayIdentifier, err = p.expectIdent(); err != nil {
				return result, err
			}
		default:
			return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes DISPLAY_IDENTIFIER)", name, keyword)
		}
	}
	if err := p.expectEndKeyword("AXIS_PTS"); err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

// parseGroup reads a GROUP block: its name/description, then any number of
// optional items in any order — the bare ROOT marker, and the
// SUB_GROUP/REF_CHARACTERISTIC/REF_MEASUREMENT/FUNCTION_LIST
// identifier-list blocks. Only REF_CHARACTERISTIC's list becomes mapdef
// category membership at conversion time; the rest are preserved on the
// model, never resolved — see docs/a2l-v0-plan.md's GROUP evidence.
func (p *parser) parseGroup() (types.A2LGroup, error) {
	name, err := p.expectIdent()
	if err != nil {
		return types.A2LGroup{}, err
	}
	description, err := p.expectString()
	if err != nil {
		return types.A2LGroup{}, err
	}
	result := types.A2LGroup{Name: name, Description: description}
	for p.peek().kind == tokIdent || p.peek().kind == tokBegin {
		if p.peek().kind == tokIdent {
			if p.peek().text != "ROOT" {
				return result, fmt.Errorf("%s: unsupported optional keyword %q (this slice only recognizes ROOT, SUB_GROUP, REF_CHARACTERISTIC, REF_MEASUREMENT, FUNCTION_LIST)", name, p.peek().text)
			}
			p.advance()
			result.Root = true
			continue
		}
		p.advance()
		keyword, err := p.expectIdent()
		if err != nil {
			return result, err
		}
		var target *[]string
		switch keyword {
		case "SUB_GROUP":
			target = &result.SubGroups
		case "REF_CHARACTERISTIC":
			target = &result.RefCharacteristics
		case "REF_MEASUREMENT":
			target = &result.RefMeasurements
		case "FUNCTION_LIST":
			target = &result.FunctionRefs
		default:
			return result, fmt.Errorf("%s: unsupported nested block %q (this slice only recognizes SUB_GROUP, REF_CHARACTERISTIC, REF_MEASUREMENT, FUNCTION_LIST)", name, keyword)
		}
		values, err := p.parseIdentList(keyword)
		if err != nil {
			return result, err
		}
		*target = values
	}
	if err := p.expectEndKeyword("GROUP"); err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

// parseIdentList reads a nested block that's just one or more identifiers
// (SUB_GROUP, REF_CHARACTERISTIC, REF_MEASUREMENT, and FUNCTION_LIST all
// share this shape), up to its matching /end keyword.
func (p *parser) parseIdentList(keyword string) ([]string, error) {
	var values []string
	for p.peek().kind == tokIdent {
		value, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := p.expectEndKeyword(keyword); err != nil {
		return nil, err
	}
	return values, nil
}
