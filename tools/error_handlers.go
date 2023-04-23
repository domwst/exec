package tools

import (
	"log"
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
	logger.Printf("Error encountered: %v\n", err)
}
