package cmd

// ToolResult be stored in
type ToolResult struct {
	ToolOutput  string   `json:"tool-output"`
	OutputFiles []string `json:"output-files"`
}

type RunStatus uint8

const (
	Enqueued RunStatus = iota
	Processing
	Processed
)

type RunResult struct {
	Status       RunStatus `json:"status"`
	ToolResultId string    `json:"result-id"`
}
