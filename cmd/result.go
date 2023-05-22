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
	Finished
)

func (s RunStatus) ToString() string {
	switch s {
	case Enqueued:
		return "enqueued"
	case Processing:
		return "processing"
	case Finished:
		return "finished"
	}
	return ""
}

type RunResult struct {
	Status       RunStatus `json:"status"`
	ToolResultId string    `json:"result-id"`
}
