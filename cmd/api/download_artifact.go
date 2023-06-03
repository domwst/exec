package main

import (
	"errors"
	"exec/common"
	"github.com/nats-io/nats.go"
	"io"
	"net/http"
	"strconv"
)

var errArtifactNotFound = errors.New("artifact not found")

func (c *connection) downloadArtifact(id string, file io.Writer, onSuccess func(size uint64)) error {
	or, err := c.osb.Get(id)
	if err != nil {
		return err
	}
	oi, err := or.Info()
	if errors.Is(err, nats.ErrObjectNotFound) {
		return errArtifactNotFound
	}
	if err != nil {
		return err
	}
	onSuccess(oi.Size)
	_, err = io.Copy(file, or)
	return err
}

func (c *connection) handleDownloadArtifact(resp http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")

	err := c.downloadArtifact(id, resp, func(size uint64) {
		resp.WriteHeader(http.StatusOK)
		resp.Header().Set("Content-Type", "application/octet-stream")
		resp.Header().Set("Content-Length", strconv.FormatUint(size, 10))
	})
	if errors.Is(err, errArtifactNotFound) {
		c.returnErrorStr(resp, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		common.HandleErrLog(err, c.logger)
		c.returnErrorStr(resp, http.StatusInternalServerError, err.Error())
		return
	}
}
