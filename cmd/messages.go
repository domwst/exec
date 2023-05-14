package cmd

import (
	"strconv"
	"strings"
)

// InheritedEnv should remain unchanged
var InheritedEnv = [...]string{"PATH"}

type InputFile struct {
	ObjectStoreId string `json:"object-store-id"`
	Extension     string `json:"extension,omitempty"`
}

type TaskMsg struct {
	InputFiles           []InputFile `json:"input-files"`
	OutputFileExtensions []string    `json:"output-file-extensions"` // List of output files extensions
	Tool                 string      `json:"tool"`
	Arguments            []string    `json:"arguments"`
	Environment          []string    `json:"environment,omitempty"`
	NotificationUrl      string      `json:"notification-url,omitempty"`
}

func getOutputFilenamePlaceholder(id int, extension string) string {
	return "<output-file#" + strconv.Itoa(id) + ">" + extension
}

func (t *TaskMsg) ReplacePlaceholderOutputs(realFilenames []string) {
	if len(realFilenames) != len(t.OutputFileExtensions) {
		panic("RealFileNames and OutputFileExtensions lengths don't match")
	}
	for id, name := range realFilenames {
		var placeholder = getOutputFilenamePlaceholder(id, t.OutputFileExtensions[id])
		for i := range t.Arguments {
			t.Arguments[i] = strings.Replace(t.Arguments[i], placeholder, name, -1)
		}
		for i := range t.Environment {
			t.Environment[i] = strings.Replace(t.Environment[i], placeholder, name, -1)
		}
	}
}

type RunStatus uint8

const (
	Enqueued RunStatus = iota
	Processing
	Processed
)

type RunResult struct {
	ToolExitCode int
	ToolOutput   string
}
