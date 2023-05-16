package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"exec/cmd"
	"exec/common"
	"flag"
	"fmt"
	"github.com/nats-io/nats.go"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const tmpPath = "/tmp"

func fetchFiles(osb nats.ObjectStore, inputFiles []cmd.InputFile, logger *log.Logger) ([]string, error) {
	var cleanup common.Cleanup
	defer cleanup.Do()
	var createdFiles []string

	for _, fileInfo := range inputFiles {
		id, ext := fileInfo.ObjectStoreId, fileInfo.Extension
		fileName := filepath.Join(tmpPath, common.GetRandomId()) + ext
		err := common.RobustGetObjectFile(osb, id, fileName)
		if err != nil {
			return nil, err
		}

		createdFiles = append(createdFiles, fileName)
		cleanup.AddAction(func() {
			err := os.Remove(fileName)
			if err != nil {
				logger.Printf("Failed to delete temporary file %s due to %+v", fileName, err)
			}
		})
	}
	cleanup.Discard()

	return createdFiles, nil
}

func createTmpFileNames(exts []string) []string {
	result := make([]string, len(exts))
	for i, ext := range exts {
		result[i] = tmpPath + common.GetRandomId() + ext
	}
	return result
}

func changeStatusToProcessing(kvb nats.KeyValue, key string, logger *log.Logger) error {
	_, _, err := common.CAS(
		kvb,
		key,
		func(curState []byte) bool {
			var r cmd.RunResult
			common.HandleErrLog(json.Unmarshal(curState, &r), logger)
			return r.Status != cmd.Enqueued
		},
		func(curState []byte) ([]byte, error) {
			var r cmd.RunResult
			common.HandleErrLog(json.Unmarshal(curState, &r), logger)
			r.Status = cmd.Processing
			return json.Marshal(r)
		},
	)
	return err
}

func worker(
	sub *nats.Subscription,
	osb nats.ObjectStore,
	kvb nats.KeyValue,
	logger *log.Logger,
	toolsPath string,
) {
	var errorCount = 0
	for {
		var c common.Cleanup
		msgRaw, err := common.RobustFetch(sub, 1, context.Background())
		if err != nil {
			logger.Printf("Error occurred: %+v\n", err)
			errorCount++
			if errorCount == 10 {
				logger.Fatalf("10 errors in a row, there must be something wrong\n")
			}
			continue
		}
		errorCount = 0
		if len(msgRaw) != 1 {
			logger.Fatalf("Expected batch of size 1 but got %d\n", len(msgRaw))
		}
		logger.Printf("Received message: %s\n", err)
		var msg cmd.TaskMsg
		err = json.Unmarshal(msgRaw[0].Data, &msg)
		common.HandleErrLog(err, logger)
		if err != nil {
			continue
		}
		go func() {
			common.HandleErrLog(changeStatusToProcessing(kvb, msg.KVId, logger), logger)
		}() // I don't care if it'll be finished after the processing of the request as long as I perform CAS inside

		inputFiles, err := fetchFiles(osb, msg.InputFiles, logger)
		c.AddAction(func() {
			for i, name := range inputFiles {
				err := os.Remove(name)
				if err == nil {
					continue
				}
				if errors.Is(err, os.ErrNotExist) {
					logger.Printf("Input file %s (#%d) was deleted by the tool", name, i)
				} else {
					logger.Printf("Failed to delete input file %s (#%d) due to %+v", name, i, err)
				}
			}
		})
		outputFiles := createTmpFileNames(msg.OutputFileExtensions)
		c.AddAction(func() {
			for i, name := range outputFiles {
				err := os.Remove(name)
				if err == nil || errors.Is(err, os.ErrNotExist) {
					continue
				}
				logger.Printf("Failed to delete output file %s (#%d) due to %+v", name, i, err)
			}
		})
		msg.ReplacePlaceholderFilenames(inputFiles, outputFiles)

		subProc := exec.Command(filepath.Join(toolsPath, msg.Tool), msg.Arguments...)

		subProc.Stdin = nil
		stderr := new(bytes.Buffer)
		subProc.Stderr = stderr
		stdout := new(bytes.Buffer)
		subProc.Stdout = stdout

		subProc.Env = msg.CreateEnv()

		common.HandleErrLog(subProc.Start(), logger)
		err = subProc.Wait()
		{
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				logger.Printf("Tool exited with non-zero code %d", exitError.ExitCode())
			} else {
				logger.Printf("Tool wait finished with an error %+v", err)
			}
		}
		if stderr.Len() != 0 {
			logger.Printf("Tool has non-empty error output \"%s\"", stderr.Bytes())
		}

		var toolResult cmd.ToolResult
		toolResult.ToolOutput = stdout.String()
		for _, name := range outputFiles {
			var idToAppend string
			id, err := common.RobustPubObjectFileRandomName(osb, name)
			if errors.Is(err, os.ErrNotExist) {
				idToAppend = ""
			} else if err != nil {
				idToAppend = fmt.Sprintf("error: %+v", err)
			} else {
				idToAppend = id.Name
			}
			toolResult.OutputFiles = append(toolResult.OutputFiles, idToAppend)
		}

		c.Do()
	}
}

func main() {
	configPath := flag.String("config-file", "worker-config.json", "Path to the worker config file")
	dotEnvPath := flag.String("env-file", ".env", "Path to .env-like file, \"-\" "+
		"or non-existing file for none")
	help := flag.Bool("help", false, "Print help")
	flag.Parse()

	if *help {
		common.Printfln("Usage: %s", os.Args[0])
		flag.PrintDefaults()
		return
	}

	env := cmd.GetEnv(*dotEnvPath)
	var workerConfig cmd.WorkerConfig
	common.HandlePanic(cmd.ParseConfigFileWithRespectToEnv(*configPath, env, &workerConfig))

	nc, err := workerConfig.ConnectionConfig.Connect()
	common.HandlePanic(err)
	defer nc.Close()

	js, err := nc.JetStream()
	common.HandlePanic(err)

	osb, err := js.ObjectStore(workerConfig.ObjectStoreBucketConfig.Name)
	common.HandlePanic(err)

	kvb, err := js.KeyValue(workerConfig.KeyValueBucketConfig.Name)

	consumerConfig := workerConfig.ConsumerConfig
	sub, err := js.PullSubscribe("", consumerConfig.Name, nats.Bind(consumerConfig.StreamName, consumerConfig.Name))
	common.HandlePanic(err)

	var wg common.WorkGroup
	for i := 0; i < workerConfig.WorkerThreads; i++ {
		i := i
		wg.Spawn(func() {
			logger := log.New(
				os.Stderr,
				fmt.Sprintf("Worker #%d: ", i),
				log.LstdFlags|log.LUTC|log.Lmsgprefix|log.Lmicroseconds,
			)
			worker(sub, osb, kvb, logger, workerConfig.PathToTools)
		})
	}

	wg.Wait()
}
