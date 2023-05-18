package common

import (
	"log"
	"runtime/debug"
)

func HandlePanic(err error) {
	if err == nil {
		return
	}
	panic(err)
}

func HandleErrLog(err error, logger *log.Logger) {
	if err == nil {
		return
	}
	logger.Printf("Encountered error %v\nAt %s", err, debug.Stack())
}
