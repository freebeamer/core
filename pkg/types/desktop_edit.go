package types

type CellOperation string

const (
	CellOperationFill         CellOperation = "fill"
	CellOperationOffset       CellOperation = "offset"
	CellOperationMultiply     CellOperation = "multiply"
	CellOperationDivide       CellOperation = "divide"
	CellOperationScalePercent CellOperation = "scale-percent"
)

type CellRange struct {
	StartRow    int `json:"startRow"`
	StartColumn int `json:"startColumn"`
	EndRow      int `json:"endRow"`
	EndColumn   int `json:"endColumn"`
}

type CellOperationRequest struct {
	Generation uint64        `json:"generation"`
	MapID      int           `json:"mapId"`
	Selection  CellRange     `json:"selection"`
	Operation  CellOperation `json:"operation"`
	Operand    float64       `json:"operand"`
}

type BatchEditOutcome struct {
	MapID          int     `json:"mapId"`
	MapTitle       string  `json:"mapTitle"`
	SelectedCells  int     `json:"selectedCells"`
	ChangedCells   int     `json:"changedCells"`
	UnchangedCells int     `json:"unchangedCells"`
	Minimum        float64 `json:"minimum"`
	Maximum        float64 `json:"maximum"`
	ChangedBytes   int     `json:"changedBytes"`
	CanUndo        bool    `json:"canUndo"`
	CanRedo        bool    `json:"canRedo"`
}

type EditOutcome struct {
	MapID        int     `json:"mapId"`
	MapTitle     string  `json:"mapTitle"`
	Row          int     `json:"row"`
	Column       int     `json:"column"`
	Index        int     `json:"index"`
	Requested    float64 `json:"requested"`
	Encoded      float64 `json:"encoded"`
	Raw          int64   `json:"raw"`
	Address      string  `json:"address"`
	ChangedBytes int     `json:"changedBytes"`
	CanUndo      bool    `json:"canUndo"`
	CanRedo      bool    `json:"canRedo"`
}

type EditDiff struct {
	MapID       int     `json:"mapId"`
	MapTitle    string  `json:"mapTitle"`
	Row         int     `json:"row"`
	Column      int     `json:"column"`
	Index       int     `json:"index"`
	Address     string  `json:"address"`
	BeforeRaw   int64   `json:"beforeRaw"`
	AfterRaw    int64   `json:"afterRaw"`
	BeforeValue float64 `json:"beforeValue"`
	AfterValue  float64 `json:"afterValue"`
}

type DiffSummary struct {
	ChangedBytes int        `json:"changedBytes"`
	Edits        []EditDiff `json:"edits"`
}
