package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"exec/cmd"
	"exec/common"
	nats2 "exec/nats"
	"flag"
	"github.com/gorilla/mux"
	"github.com/nats-io/nats.go"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

type connection struct {
	publisher    common.Publisher[cmd.TaskMsg]
	resultKvb    common.KeyValueBucket[cmd.RunResult]
	osb          nats.ObjectStore
	tasksSubject string
	logger       *log.Logger
}

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

func createErrResponse(errMsg string) []byte {
	type Err struct {
		Error string `json:"error"`
	}
	data, e := json.Marshal(&Err{Error: errMsg})
	common.HandlePanic(e)
	return data
}

const (
	maxMemory     = 128 << 10
	maxSourceSize = 64 << 10
)

func (c *connection) returnErrorStr(
	resp http.ResponseWriter,
	status int,
	errMsg string,
) {
	resp.WriteHeader(status)
	_, err := resp.Write(createErrResponse(errMsg))
	common.HandleErrLog(err, c.logger)
}

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

func toString(s cmd.RunStatus) string {
	switch s {
	case cmd.Enqueued:
		return "enqueued"
	case cmd.Processing:
		return "processing"
	case cmd.Finished:
		return "finished"
	}
	return ""
}

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

func (c *connection) handleGetStatus(resp http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	if id == "" {
		c.returnErrorStr(resp, http.StatusBadRequest, "id required")
		return
	}
	status, result, err := c.getStatus(id)
	if err == nats.ErrKeyNotFound || (result != nil && len(result.OutputFiles) != 2) {
		c.returnErrorStr(resp, http.StatusNotFound, "run id not found")
		return
	}
	if err != nil {
		c.returnErrorStr(resp, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	type Result struct {
		Status   string `json:"status"`
		BinaryId string `json:"binary-id,omitempty"`
		ErrLogId string `json:"error-log-id,omitempty"`
	}
	res := Result{
		Status: toString(status),
	}
	if result != nil {
		res.BinaryId = result.OutputFiles[0]
		res.ErrLogId = result.OutputFiles[1]
	}

	resp.WriteHeader(http.StatusOK)
	data, err := json.Marshal(&res)
	common.HandleErrLog(err, c.logger)
	_, err = resp.Write(data)
	common.HandleErrLog(err, c.logger)
}

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
	if id == "" {
		c.returnErrorStr(resp, http.StatusBadRequest, "id required")
		return
	}

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

func main() {
	configPath := flag.String("config-file", "worker-config.json", "Path to the worker config file")
	help := flag.Bool("help", false, "Print help")
	flag.Parse()

	if *help {
		common.Printfln("Usage: %s", os.Args[0])
		flag.PrintDefaults()
		return
	}

	env := cmd.ParseEnvironment(os.Environ())
	var workerConfig cmd.WorkerConfig
	common.HandlePanic(cmd.ParseConfigFileWithRespectToEnv(*configPath, env, &workerConfig))

	nc, err := workerConfig.ConnectionConfig.Connect()
	common.HandlePanic(err)
	defer nc.Close()

	js, err := nc.JetStream()
	common.HandlePanic(err)

	kvb, err := js.KeyValue(workerConfig.KeyValueBucketConfig.Name)
	common.HandlePanic(err)
	osb, err := js.ObjectStore(workerConfig.ObjectStoreBucketConfig.Name)
	common.HandlePanic(err)

	logger := log.New(
		os.Stderr,
		"Api: ",
		log.LstdFlags|log.LUTC|log.Lmsgprefix|log.Lmicroseconds,
	)

	conn := &connection{
		publisher:    nats2.NewPublisherWrapper[cmd.TaskMsg](js, &common.JsonSerializer[cmd.TaskMsg]{}),
		resultKvb:    nats2.NewKeyValueTypedWrapper[cmd.RunResult](kvb, &common.JsonSerializer[cmd.RunResult]{}),
		osb:          osb,
		tasksSubject: workerConfig.ConsumerConfig.StreamName,
		logger:       logger,
	}

	r := mux.NewRouter()
	r.NewRoute().Methods(http.MethodPost).Path("/submit").HandlerFunc(conn.handleSubmit)
	r.NewRoute().Methods(http.MethodGet).Path("/compileStatus").HandlerFunc(conn.handleGetStatus)
	r.NewRoute().Methods(http.MethodGet).Path("/downloadArtifact").HandlerFunc(conn.handleDownloadArtifact)

	err = http.ListenAndServe(":8000", r)
	common.HandleErrLog(err, logger)
}
