package types

type MapShape string

const (
	ShapeScalar  MapShape = "scalar"
	ShapeOneAxis MapShape = "one-axis"
	ShapeMatrix  MapShape = "matrix"
	ShapeInvalid MapShape = "invalid"
)

type MapSummary struct {
	ID         int      `json:"id"`
	Title      string   `json:"title"`
	Categories []string `json:"categories"`
	Shape      MapShape `json:"shape"`
}

type MapFilter struct {
	Query    string `json:"query"`
	Category string `json:"category"`
}

type MapPage struct {
	Generation uint64       `json:"generation"`
	Total      int          `json:"total"`
	Categories []string     `json:"categories"`
	Maps       []MapSummary `json:"maps"`
}

type AxisDetail struct {
	Units       string `json:"units"`
	Formula     string `json:"formula"`
	Addressed   bool   `json:"addressed"`
	ElementBits int    `json:"elementBits"`
}

type MapDetail struct {
	ID int `json:"id"`
	// ParameterID is the definition's stable parameter ID that profile
	// bindings refer to; it is empty when the definition provides none.
	ParameterID string     `json:"parameterId,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Categories  []string   `json:"categories"`
	Shape       MapShape   `json:"shape"`
	Diagnostics string     `json:"diagnostics,omitempty"`
	Rows        int        `json:"rows"`
	Columns     int        `json:"columns"`
	X           AxisDetail `json:"x"`
	Y           AxisDetail `json:"y"`
	Z           AxisDetail `json:"z"`

	XValues    []float64   `json:"xValues,omitempty"`
	XRaw       []float64   `json:"xRaw,omitempty"`
	XAddresses []string    `json:"xAddresses,omitempty"`
	YValues    []float64   `json:"yValues,omitempty"`
	YRaw       []float64   `json:"yRaw,omitempty"`
	YAddresses []string    `json:"yAddresses,omitempty"`
	ZValues    [][]float64 `json:"zValues,omitempty"`
	ZRaw       [][]float64 `json:"zRaw,omitempty"`
	ZAddresses [][]string  `json:"zAddresses,omitempty"`
}
