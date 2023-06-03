package main

import (
	"encoding/json"
	"exec/cmd"
	"exec/common"
	"net/http"
)

// TODO: Possible failure because of absence of the OS item
func (c *connection) run(osId string) (string, error) {
	runId := common.GetRandomId()
	_, err := c.resultKvb.Create(runId, &cmd.RunResult{
		Status: cmd.Enqueued,
	})
	if err != nil {
		return "", err
	}
	task := cmd.TaskMsg{
		InputFiles: []cmd.InputFile{
			{osId, ".cpp"},
		},
		OutputFileExtensions: []string{".out", ".log"},
		Tool:                 "run",
		Arguments:            []string{"<input-file#0>", "/dev/null", "<output-file#0>", "<output-file#1>"},
		Environment:          []string{},
		NotificationUrl:      "",
		KVId:                 runId,
	}
	err = c.publisher.PublishSync(c.tasksSubject, &task)
	if err != nil {
		return "", err
	}
	return runId, nil
}

func (c *connection) handleRun(resp http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	runId, err := c.run(id)
	if err != nil {
		c.returnErrorStr(resp, http.StatusInternalServerError, err.Error())
		return
	}
	type Response struct {
		RunId string `json:"id"`
	}
	data, err := json.Marshal(&Response{
		RunId: runId,
	})
	common.HandleErrLog(err, c.logger)

	resp.WriteHeader(http.StatusOK)
	_, err = resp.Write(data)
	common.HandleErrLog(err, c.logger)
}
