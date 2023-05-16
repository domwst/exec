package cmd

import (
	"os"
	"strconv"
	"strings"
)

// inheritedEnv should remain unchanged
var inheritedEnvPrefixes = [...]string{"PATH="}

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
	KVId                 string      `json:"key-value-id"`
}

// TaskMsg.Arguments may contain placeholders for input and output files:
// Arguments = ["<input-file#0>", "-o", "<output-file#0>"]
// Which will be replaced with ["source.cpp", "-o", "executable"]
// if input extensions are ".cpp" and "" for input and output files respectively
//
// The placeholders can appear as a part of an argument:
// Arguments = ["--input=<input-file#0>", "--output=<output-file#0>"]
// Placeholders are purposely verbose and odd-looking to decrease change
// of collision with "real" arguments

func getInputFilenamePlaceholder(id int) string {
	return "<input-file#" + strconv.Itoa(id) + ">"
}

func getOutputFilenamePlaceholder(id int) string {
	return "<output-file#" + strconv.Itoa(id) + ">"
}

func (t *TaskMsg) replaceInArgsAndEnv(old string, new string) {
	for i := range t.Arguments {
		t.Arguments[i] = strings.Replace(t.Arguments[i], old, new, -1)
	}
	for i := range t.Environment {
		t.Environment[i] = strings.Replace(t.Environment[i], old, new, -1)
	}
}

func (t *TaskMsg) ReplacePlaceholderFilenames(inputFileNames []string, outputFileNames []string) {
	if len(inputFileNames) != len(t.InputFiles) {
		panic("inputFileNames and InputFiles lengths don't match")
	}
	if len(outputFileNames) != len(t.OutputFileExtensions) {
		panic("outputFileNames and OutputFileExtensions lengths don't match")
	}
	for id, name := range inputFileNames {
		t.replaceInArgsAndEnv(getInputFilenamePlaceholder(id), name)
	}
	for id, name := range outputFileNames {
		t.replaceInArgsAndEnv(getOutputFilenamePlaceholder(id), name)
	}
}

func (t *TaskMsg) CreateEnv() []string {
	var inherited []string
	for _, e := range os.Environ() {
		for _, pref := range inheritedEnvPrefixes {
			if strings.HasPrefix(e, pref) {
				inherited = append(inherited, e)
			}
		}
	}
	return append(inherited, t.Environment...)
}
