package main

import (
	"exec/cmd"
	"exec/common"
	"github.com/nats-io/nats.go"
	"log"
	"net/http"
)

type connection struct {
	publisher    common.Publisher[cmd.TaskMsg]
	resultKvb    common.KeyValueBucket[cmd.RunResult]
	osb          nats.ObjectStore
	tasksSubject string
	logger       *log.Logger
}

func (c *connection) returnErrorStr(
	resp http.ResponseWriter,
	status int,
	errMsg string,
) {
	if status == http.StatusInternalServerError {
		c.logger.Printf("Error: %s", errMsg)
	}
	resp.WriteHeader(status)
	_, err := resp.Write(CreateErrResponse(errMsg))
	common.HandleErrLog(err, c.logger)
}
