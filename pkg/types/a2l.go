package types

// A2LDefinition is the normalized result of parsing an A2L (ASAM MCD-2 MC /
// ASAP2) file. See docs/a2l-v0-plan.md for exactly what this V0 slice reads.
type A2LDefinition struct {
	Project A2LProject
	RawText []byte
}

type A2LProject struct {
	Name        string
	Description string
	Module      A2LModule
}

type A2LModule struct {
	Name            string
	Description     string
	CompuMethods    []A2LCompuMethod
	CompuVtabs      []A2LCompuVtab
	CompuVtabRanges []A2LCompuVtabRange
	CompuTabs       []A2LCompuTab
	RecordLayouts   []A2LRecordLayout
	Characteristics []A2LCharacteristic
	AxisPts         []A2LAxisPts
	Groups          []A2LGroup
	// ByteOrder is MOD_COMMON's optional BYTE_ORDER value (raw A2L token,
	// e.g. "MSB_LAST"), empty if MOD_COMMON or its BYTE_ORDER is absent.
	// It is the module-wide default; a CHARACTERISTIC's own ByteOrder
	// overrides it.
	ByteOrder string
}

// A2LGroup is a GROUP block: a named collection of CHARACTERISTICs (and,
// per real evidence, MEASUREMENTs and FUNCTIONs this slice doesn't parse),
// analogous to XDF's <CATEGORY> and converted to mapdef's categories.
// GROUP supports a real hierarchy (SUB_GROUP) that mapdef's flat
// categories field cannot represent — see docs/a2l-v0-plan.md's GROUP
// evidence.
type A2LGroup struct {
	Name        string
	Description string
	// Root is true when the bare ROOT keyword is present: an ASAM
	// convention marking a group as a top-level entry point into the
	// SUB_GROUP tree. Preserved, not used to filter which groups convert.
	Root bool
	// RefCharacteristics is REF_CHARACTERISTIC's identifier list: the
	// CHARACTERISTICs that are direct members of this group. Only this
	// list becomes category membership — SubGroups is not expanded
	// transitively.
	RefCharacteristics []string
	// SubGroups is SUB_GROUP's identifier list: nested child A2LGroup
	// names. Preserved, not flattened into RefCharacteristics.
	SubGroups []string
	// RefMeasurements is REF_MEASUREMENT's identifier list. Preserved,
	// never resolved — this slice does not parse MEASUREMENT.
	RefMeasurements []string
	// FunctionRefs is FUNCTION_LIST's identifier list. Preserved, never
	// resolved — this slice does not parse FUNCTION.
	FunctionRefs []string
}

// A2LAxisPts is a standalone, shareable AXIS_PTS block: a top-level axis
// definition a CHARACTERISTIC's AXIS_DESCR references via AXIS_PTS_REF
// (Attribute COM_AXIS) instead of storing its breakpoints inline.
type A2LAxisPts struct {
	Name              string
	Description       string
	Address           uint64
	InputQuantity     string
	Deposit           string // RECORD_LAYOUT reference
	MaxDiff           float64
	Conversion        string // COMPU_METHOD reference, or NO_COMPU_METHOD
	MaxAxisPoints     int
	LowerLimit        float64
	UpperLimit        float64
	DisplayIdentifier string
}

// A2LCompuMethod is a COMPU_METHOD block. ConversionType IDENTICAL,
// LINEAR, TAB_VERB, RAT_FUNC, FORM, TAB_INTP, and TAB_NOINTP are all
// interpreted by this slice; others are preserved (Name, Description,
// ConversionType, Format, Unit) but rejected at conversion time with an
// explicit error rather than guessed.
type A2LCompuMethod struct {
	Name            string
	Description     string
	ConversionType  string
	Format          string
	Unit            string
	CoeffsLinearA   float64
	CoeffsLinearB   float64
	HasCoeffsLinear bool
	// CoeffsA..CoeffsF are RAT_FUNC's six COEFFS values: phys =
	// (a*int^2 + b*int + c) / (d*int^2 + e*int + f). HasCoeffs indicates
	// COEFFS was present.
	CoeffsA, CoeffsB, CoeffsC, CoeffsD, CoeffsE, CoeffsF float64
	HasCoeffs                                            bool
	// Formula is FORM's forward FORMULA expression (F_x), preserved
	// verbatim in ASAM's own variable-naming convention (X1, ...); the
	// converter is responsible for translating it. Empty if absent.
	Formula string
	// FormulaInv is FORM's optional FORMULA_INV (G_x). Preserved, never
	// used — this project always derives an edit-time inverse
	// algebraically from Formula/Conversion itself, never from a
	// separately supplied inverse formula.
	FormulaInv string
	// CompuTabRef is TAB_VERB/TAB_INTP/TAB_NOINTP's optional
	// COMPU_TAB_REF value: for TAB_VERB, the referenced A2LCompuVtab.Name
	// (or an A2LCompuVtabRange.Name); for TAB_INTP/TAB_NOINTP, the
	// referenced A2LCompuTab.Name. Empty otherwise.
	CompuTabRef string
}

// A2LCompuVtab is a COMPU_VTAB block: an exact value-to-text lookup table a
// TAB_VERB COMPU_METHOD references via COMPU_TAB_REF.
type A2LCompuVtab struct {
	Name        string
	Description string
	Entries     []A2LCompuVtabEntry
	// DefaultValue is the optional DEFAULT_VALUE text shown for a raw value
	// not in Entries. Preserved, not applied by any consumer yet.
	DefaultValue    string
	HasDefaultValue bool
}

// A2LCompuVtabEntry is one exact value/text pair of a COMPU_VTAB.
type A2LCompuVtabEntry struct {
	Value int
	Text  string
}

// A2LCompuVtabRange is a COMPU_VTAB_RANGE block: a range-keyed text lookup
// table, the same COMPU_TAB_REF a TAB_VERB COMPU_METHOD can point at instead
// of a COMPU_VTAB — the COMPU_METHOD itself doesn't distinguish which shape
// its reference resolves to (see docs/a2l-v0-plan.md's TAB_VERB / COMPU_VTAB
// evidence). Parsed and preserved, but converting a Conversion that resolves
// to one is rejected: mapdef's staticValues field is keyed by a single
// index, not a range, so this has no mapdef representation yet.
type A2LCompuVtabRange struct {
	Name        string
	Description string
	Entries     []A2LCompuVtabRangeEntry
	// DefaultValue is the optional DEFAULT_VALUE text shown for a raw value
	// in no entry's range. Preserved, not applied by any consumer yet.
	DefaultValue    string
	HasDefaultValue bool
}

// A2LCompuVtabRangeEntry is one inclusive [LowerValue, UpperValue] -> Text
// entry of a COMPU_VTAB_RANGE.
type A2LCompuVtabRangeEntry struct {
	LowerValue int
	UpperValue int
	Text       string
}

// A2LCompuTab is a COMPU_TAB block: a numeric value-to-value lookup table
// a TAB_INTP/TAB_NOINTP COMPU_METHOD references via COMPU_TAB_REF — a
// different block from COMPU_VTAB (text output): this one's OutVal is
// numeric, converting to mapdef's "freehorse-lookup-table-v1" Conversion
// shape (mapdef 1.3+) rather than staticValues. Interpolated is true for
// TAB_INTP (linear interpolation between entries), false for TAB_NOINTP
// (exact match only) — see docs/a2l-v0-plan.md's COMPU_TAB evidence.
type A2LCompuTab struct {
	Name         string
	Description  string
	Interpolated bool
	Entries      []A2LCompuTabEntry
	// DefaultValue is the optional DEFAULT_VALUE_NUMERIC output for a raw
	// value not covered by Entries (outside its range, or — for
	// TAB_NOINTP — not an exact match).
	DefaultValue    float64
	HasDefaultValue bool
}

// A2LCompuTabEntry is one exact InVal/OutVal pair of a COMPU_TAB.
type A2LCompuTabEntry struct {
	InVal  float64
	OutVal float64
}

// A2LRecordLayout is a RECORD_LAYOUT block: a name plus its typed entries.
type A2LRecordLayout struct {
	Name    string
	Entries []A2LRecordLayoutEntry
}

// A2LRecordLayoutEntry is one positioned field within a RECORD_LAYOUT.
// Position is an ordinal sequence number (1st, 2nd, ...), not a byte
// offset: the actual byte offset of each entry is the cumulative size of
// every lower-positioned entry, computed during conversion once the axis
// point counts (from the CHARACTERISTIC's AXIS_DESCRs) that size
// AXIS_PTS_X/Y and FNC_VALUES are known.
type A2LRecordLayoutEntry struct {
	// Role is FNC_VALUES, AXIS_PTS_X, AXIS_PTS_Y, NO_AXIS_PTS_X,
	// NO_AXIS_PTS_Y, AXIS_RESCALE_X, NO_RESCALE_X, or RESERVED.
	Role      string
	Position  int
	DataType  string
	IndexMode string // empty for NO_AXIS_PTS_X/Y, NO_RESCALE_X, and RESERVED, which have none
	AddrType  string // empty for NO_AXIS_PTS_X/Y, NO_RESCALE_X, and RESERVED, which have none
	// RescalePairs is AXIS_RESCALE_X's MaxNumberOfRescalePairs positional
	// field (how many (position, value) pairs this entry's storage holds).
	// Zero for every other Role.
	RescalePairs int
}

// A2LExtendedLimits is a CHARACTERISTIC's optional EXTENDED_LIMITS pair.
type A2LExtendedLimits struct {
	Low  float64
	High float64
}

// A2LCharacteristic is a CHARACTERISTIC block. This slice converts Type
// "VALUE" (scalar), "CURVE" (one AXIS_DESCR), and "MAP" (two AXIS_DESCRs,
// first X then Y by declaration order) characteristics.
type A2LCharacteristic struct {
	Name              string
	Description       string
	Type              string
	Address           uint64
	Deposit           string
	MaxDiff           float64
	Conversion        string
	LowerLimit        float64
	UpperLimit        float64
	Format            string
	DisplayIdentifier string
	ExtendedLimits    *A2LExtendedLimits
	// ByteOrder is this characteristic's own optional BYTE_ORDER value (raw
	// A2L token), empty if not declared. It overrides the module's
	// MOD_COMMON default when present.
	ByteOrder string
	// AxisDescrs holds one nested AXIS_DESCR block per axis: empty for
	// VALUE and VAL_BLK, one entry for CURVE, two (X then Y) for MAP.
	AxisDescrs []A2LAxisDescr
	// MatrixDim is the optional MATRIX_DIM keyword's three values, required
	// for and only meaningful on a Type VAL_BLK characteristic (which has
	// no AXIS_DESCR of its own). Nil otherwise.
	MatrixDim *A2LMatrixDim
}

// A2LMatrixDim is MATRIX_DIM's three positional values (x, y, z), giving a
// VAL_BLK characteristic's shape directly since it has no AXIS_DESCR. Only
// z == 1 (a 1- or 2-dimensional block) is supported in this slice — see
// docs/a2l-v0-plan.md's VAL_BLK evidence.
type A2LMatrixDim struct {
	X int
	Y int
	Z int
}

// A2LAxisDescr is one CHARACTERISTIC axis (one nested AXIS_DESCR block).
// Attribute "STD_AXIS" (breakpoints stored directly in the enclosing
// RECORD_LAYOUT), "COM_AXIS" (breakpoints in a shared, standalone AXIS_PTS
// referenced by AxisPtsRef), "FIX_AXIS" (breakpoints computed from
// FixAxisParDist/FixAxisParList, no on-disk storage at all), "CURVE_AXIS"
// (breakpoints borrowed from another CHARACTERISTIC's own axis, referenced
// by CurveAxisRef), and "RES_AXIS" (breakpoints in a shared, standalone
// AXIS_PTS storing rescale pairs, referenced by AxisPtsRef exactly like
// COM_AXIS, but only when its pair count matches MaxAxisPoints — see
// docs/a2l-v0-plan.md's RES_AXIS evidence) are all converted.
type A2LAxisDescr struct {
	Attribute string
	// InputQuantity is a MEASUREMENT reference. This slice does not parse
	// MEASUREMENT, so it is preserved but never resolved.
	InputQuantity string
	// Conversion is a COMPU_METHOD reference, or the NO_COMPU_METHOD
	// sentinel, exactly like A2LCharacteristic.Conversion. For a COM_AXIS/
	// RES_AXIS this is authoritative over the referenced AXIS_PTS's own
	// Conversion, which real files use only as a placeholder. For a
	// CURVE_AXIS this is itself always just a NO_COMPU_METHOD placeholder
	// ("CURVE_AXIS have no input conversion" per real evidence); the
	// referenced CHARACTERISTIC's own axis Conversion is authoritative
	// instead — see docs/a2l-v0-plan.md.
	Conversion    string
	MaxAxisPoints int
	LowerLimit    float64
	UpperLimit    float64
	// Monotony is the optional MONOTONY keyword's value (e.g.
	// "STRICT_INCREASE"), empty if absent. Parsed, not yet enforced.
	Monotony string
	// AxisPtsRef is the optional AXIS_PTS_REF keyword's value: the
	// referenced A2LAxisPts.Name for a COM_AXIS or RES_AXIS. Empty
	// otherwise.
	AxisPtsRef string
	// FixAxisParDist is the optional FIX_AXIS_PAR_DIST keyword's three
	// values, used to compute a FIX_AXIS's breakpoints as
	// Offset + i*Distance for i in [0, NumberPts). Nil if absent.
	FixAxisParDist *A2LFixAxisParDist
	// FixAxisParList is the optional FIX_AXIS_PAR_LIST block's literal
	// breakpoint values, used directly as a FIX_AXIS's raw axis points.
	// Nil if absent. At most one of FixAxisParDist/FixAxisParList is set
	// for a FIX_AXIS; neither is set for any other Attribute.
	FixAxisParList []int
	// CurveAxisRef is the optional CURVE_AXIS_REF keyword's value: the
	// referenced A2LCharacteristic.Name for a CURVE_AXIS. Empty otherwise.
	CurveAxisRef string
}

// A2LFixAxisParDist is FIX_AXIS_PAR_DIST's three positional values:
// breakpoint i (for i in [0, NumberPts)) is Offset + i*Distance.
type A2LFixAxisParDist struct {
	Offset    int
	Distance  int
	NumberPts int
}
