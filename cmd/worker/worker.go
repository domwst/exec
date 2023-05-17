package main

import (
	"bytes"
	"context"
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

// TODO: Parallelize
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

func changeStatusToProcessing(kvb nats.KeyValue, key string, logger *log.Logger) {
	_, _, err := common.CAS(
		kvb,
		key,
		func(curState *cmd.RunResult) (bool, error) {
			return curState.Status != cmd.Enqueued, nil
		},
		func(curState *cmd.RunResult) error {
			curState.Status = cmd.Processing
			return nil
		},
	)
	common.HandleErrLog(err, logger)
}

// func notify(notificationUrl string, id string, status cmd.RunStatus, logger *log.Logger) {
func notify(_ string, _ string, _ cmd.RunStatus, _ *log.Logger) {
}

func uploadResultsAndNotify(
	osb nats.ObjectStore,
	kvb nats.KeyValue,
	outputFiles []string,
	stdout string,
	msg *cmd.TaskMsg,
	logger *log.Logger,
) error {
	var toolResult cmd.ToolResult
	toolResult.ToolOutput = stdout
	toolResult.OutputFiles = make([]string, len(outputFiles))
	var wg common.WorkGroup
	for i, name := range outputFiles {
		i := i
		wg.Spawn(func() {
			var idToWrite string
			id, err := common.RobustPubObjectFileRandomName(osb, name)
			if errors.Is(err, os.ErrNotExist) {
				idToWrite = ""
			} else if err != nil {
				idToWrite = fmt.Sprintf("error: %+v", err)
			} else {
				idToWrite = id.Name
			}
			toolResult.OutputFiles[i] = idToWrite
		})
	}
	wg.Wait()

	var runResult cmd.RunResult
	runResult.Status = cmd.Finished
	{
		object, err := common.TypedRobustPutObjectRandomName(osb, &toolResult)
		runResult.ToolResultId = object.Name
		common.HandleErrLog(err, logger)
		if err != nil {
			return err
		}
	}
	_, _, err := common.CAS(
		kvb,
		msg.KVId,
		func(currentResult *cmd.RunResult) (bool, error) {
			return currentResult.Status == cmd.Finished, nil
		},
		func(result *cmd.RunResult) error {
			*result = runResult
			return nil
		},
	)
	common.HandleErrLog(err, logger)
	if err != nil {
		return err
	}
	notify(msg.NotificationUrl, msg.KVId, cmd.Finished, logger)
	return nil
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
		var fileCleanup common.Cleanup
		{
			msgs, err := common.TypedRobustFetch[cmd.TaskMsg](sub, 1, context.Background())
			if err != nil {
				logger.Printf("Error occurred: %+v\n", err)
				errorCount++
				if errorCount == 10 {
					logger.Fatalf("10 errors in a row, there must be something wrong\n")
				}
				goto cleanup
			}
			errorCount = 0
			if len(msgs) != 1 {
				logger.Fatalf("Expected batch of size 1 but got %d\n", len(msgs))
			}
			msg := msgs[0]
			content := msg.Content
			logger.Printf("Received message: %+v\n", msg)
			if err != nil {
				goto cleanup
			}
			go func() {
				changeStatusToProcessing(kvb, content.KVId, logger)
				notify(content.NotificationUrl, content.KVId, cmd.Processing, logger)
			}() // I don't care if it'll be finished after the processing of the request as long as I perform CAS inside

			inputFiles, err := fetchFiles(osb, content.InputFiles, logger)
			if err != nil {
				logger.Printf("Failed to download input files: \"%+v\"", err)
				goto cleanup
			}
			fileCleanup.AddAction(func() {
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
			outputFiles := createTmpFileNames(content.OutputFileExtensions)
			fileCleanup.AddAction(func() {
				for i, name := range outputFiles {
					err := os.Remove(name)
					if err == nil || errors.Is(err, os.ErrNotExist) {
						continue
					}
					logger.Printf("Failed to delete output file %s (#%d) due to %+v", name, i, err)
				}
			})
			content.ReplacePlaceholderFilenames(inputFiles, outputFiles)

			subProc := exec.Command(filepath.Join(toolsPath, content.Tool), content.Arguments...)

			subProc.Stdin = nil
			stderr := new(bytes.Buffer)
			subProc.Stderr = stderr
			stdout := new(bytes.Buffer)
			subProc.Stdout = stdout

			subProc.Env = content.CreateEnv()

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
			err = uploadResultsAndNotify(osb, kvb, outputFiles, stdout.String(), content, logger)

			if err != nil {
				goto cleanup
			}
			common.HandleErrLog(msg.AckSync(), logger)
		}

	cleanup:
		fileCleanup.Do()
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

	osb, err := js.ObjectStore(workerConfig.ObjectStoreBucketConfig.Name)
	common.HandlePanic(err)

	kvb, err := js.KeyValue(workerConfig.KeyValueBucketConfig.Name)
	common.HandlePanic(err)

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
