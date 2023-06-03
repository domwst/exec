package main

import (
	"bytes"
	"encoding/json"
	"exec/cmd"
	"exec/common"
	nats2 "exec/nats"
	"io"
	"net/http"
)

func (c *connection) submit(file io.Reader) (string, string, error) {
	oi, err := nats2.RobustPutObject(c.osb, file, common.GetRandomId())
	if err != nil {
		return "", "", err
	}

	id := common.GetRandomId()
	_, err = c.resultKvb.Create(id, &cmd.RunResult{
		Status: cmd.Enqueued,
	})
	if err != nil {
		return "", "", err
	}

	task := cmd.TaskMsg{
		InputFiles: []cmd.InputFile{
			{oi.Name, ".cpp"},
		},
		OutputFileExtensions: []string{"", ".log"},
		Tool:                 "clang_compile",
		Arguments:            []string{"<input-file#0>", "<output-file#0>", "<output-file#1>"},
		Environment:          []string{},
		NotificationUrl:      "",
		KVId:                 id,
	}
	err = c.publisher.PublishSync(c.tasksSubject, &task)
	if err != nil {
		return "", "", err
	}
	return id, oi.Name, nil
}

const (
	maxMemory     = 128 << 10
	maxSourceSize = 64 << 10
)

func (c *connection) handleSubmit(resp http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(maxMemory)
	if err != nil {
		c.returnErrorStr(resp, http.StatusBadRequest, err.Error())
		return
	}
	file, fh, err := req.FormFile("file")
	if fh.Size > maxSourceSize {
		c.returnErrorStr(resp, http.StatusBadRequest, "Max source file size is 64Kb")
		return
	}
	var buf bytes.Buffer
	ln, err := io.Copy(&buf, file)
	if err != nil {
		c.returnErrorStr(resp, http.StatusInternalServerError, "Failed to load source file: "+err.Error())
		return
	}
	if ln != fh.Size {
		c.returnErrorStr(resp, http.StatusInternalServerError, "Failed to load full file")
		return
	}

	id, srcId, err := c.submit(&buf)
	if err != nil {
		c.returnErrorStr(resp, http.StatusInternalServerError, "Failed to submit: "+err.Error())
		return
	}

	type Ok struct {
		Id    string `json:"id"`
		SrcId string `json:"src-id"`
	}
	resp.WriteHeader(http.StatusOK)
	data, err := json.Marshal(&Ok{
		Id:    id,
		SrcId: srcId,
	})
	common.HandleErrLog(err, c.logger)
	_, err = resp.Write(data)
	common.HandleErrLog(err, c.logger)
}
