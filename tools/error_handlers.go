package tools

import (
	"os"
)

func HandlePanic(err error) {
	if err == nil {
		return
	}
	panic(err)
}

func HandleErrLog(err error) {
	if err == nil {
		return
	}
	_, err = Fprintfln(os.Stderr, "Error encountered: %v", err)
	HandlePanic(err)
}
