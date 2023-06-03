package main

import (
	"exec/cmd"
	"exec/common"
	nats2 "exec/nats"
	"flag"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
)

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
	r.NewRoute().Methods(http.MethodGet).Path("/compileStatus").HandlerFunc(
		RequireKey("id", conn.handleGetCompilationStatus),
	)
	r.NewRoute().Methods(http.MethodPost).Path("/run").HandlerFunc(
		RequireKey("id", conn.handleRun),
	)
	r.NewRoute().Methods(http.MethodGet).Path("/runStatus").HandlerFunc(
		RequireKey("id", conn.handleGetRunStatus),
	)
	r.NewRoute().Methods(http.MethodGet).Path("/downloadArtifact").HandlerFunc(
		RequireKey("id", conn.handleDownloadArtifact),
	)

	err = http.ListenAndServe(":8000", r)
	common.HandleErrLog(err, logger)
}
