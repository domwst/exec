package main

import (
	"encoding/json"
	"errors"
	"exec/cmd"
	"exec/common"
	nats2 "exec/nats"
	"github.com/nats-io/nats.go"
	"net/http"
)

func (c *connection) getStatus(id string) (cmd.RunStatus, *cmd.ToolResult, error) {
	status, err := c.resultKvb.Get(id)
	if err != nil {
		return cmd.Enqueued, nil, err
	}
	rStatus := status.Value().Status
	if rStatus != cmd.Finished {
		return rStatus, nil, nil
	}
	toolResult, err := nats2.TypedRobustGetObject[cmd.ToolResult](c.osb, status.Value().ToolResultId, &common.JsonSerializer[cmd.ToolResult]{})
	if err != nil {
		return rStatus, nil, err
	}
	return rStatus, toolResult, nil
}

func (c *connection) genericHandleGetStatus(
	resp http.ResponseWriter,
	req *http.Request,
	notFoundMsg string,
	expectedOutputFiles int,
	getStatusImpl func(cmd.RunStatus, *cmd.ToolResult) []byte,
) {
	id := req.URL.Query().Get("id")
	status, result, err := c.getStatus(id)
	if errors.Is(err, nats.ErrKeyNotFound) || (result != nil && len(result.OutputFiles) != expectedOutputFiles) {
		c.returnErrorStr(resp, http.StatusNotFound, notFoundMsg)
		return
	}
	if err != nil {
		c.returnErrorStr(resp, http.StatusInternalServerError, "error: "+err.Error())
		return
	}
	data := getStatusImpl(status, result)

	resp.WriteHeader(http.StatusOK)
	_, err = resp.Write(data)
	common.HandleErrLog(err, c.logger)
}

func (c *connection) handleGetCompilationStatusImpl(status cmd.RunStatus, result *cmd.ToolResult) []byte {
	type Result struct {
		Status   string `json:"status"`
		BinaryId string `json:"binary-id,omitempty"`
		ErrLogId string `json:"error-log-id,omitempty"`
		Stats    string `json:"stats,omitempty"`
	}
	res := Result{
		Status: status.ToString(),
	}
	if result != nil {
		res.BinaryId = result.OutputFiles[0]
		res.ErrLogId = result.OutputFiles[1]
		res.Stats = result.ToolOutput
	}

	data, err := json.Marshal(&res)
	common.HandleErrLog(err, c.logger)
	return data
}

func (c *connection) handleGetRunStatusImpl(status cmd.RunStatus, result *cmd.ToolResult) []byte {
	type Result struct {
		Status     string `json:"status"`
		OutputId   string `json:"stdout-id,omitempty"`
		ErrorLogId string `json:"stderr-id,omitempty"`
		Stats      string `json:"stats,omitempty"`
	}
	res := Result{
		Status: status.ToString(),
	}
	if result != nil {
		res.OutputId = result.OutputFiles[0]
		res.ErrorLogId = result.OutputFiles[1]
		res.Stats = result.ToolOutput
	}
	data, err := json.Marshal(&res)
	common.HandleErrLog(err, c.logger)
	return data
}

func (c *connection) handleGetCompilationStatus(resp http.ResponseWriter, req *http.Request) {
	c.genericHandleGetStatus(resp, req, "submit id not found", 2, c.handleGetCompilationStatusImpl)
}

func (c *connection) handleGetRunStatus(resp http.ResponseWriter, req *http.Request) {
	c.genericHandleGetStatus(resp, req, "run id not found", 2, c.handleGetRunStatusImpl)
}
