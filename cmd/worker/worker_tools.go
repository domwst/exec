package main

import (
	"errors"
	"exec/cmd"
	"exec/common"
	nats2 "exec/nats"
	"fmt"
	"github.com/nats-io/nats.go"
	"log"
	"os"
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
		fileName := filepath.Join(tmpPath, common.GetRandomId()+ext)
		err := nats2.RobustGetObjectFile(osb, id, fileName)
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
		result[i] = filepath.Join(tmpPath, common.GetRandomId()+ext)
	}
	return result
}

func changeStatusToProcessing(kvb common.KeyValueBucket[cmd.RunResult], key string, logger *log.Logger) {
	_, _, err := kvb.CAS(
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
	kvb common.KeyValueBucket[cmd.RunResult],
	outputFiles []string,
	stdout string,
	msg *cmd.TaskMsg,
	logger *log.Logger,
	serializer common.Serializer[cmd.ToolResult],
) error {
	var toolResult cmd.ToolResult
	toolResult.ToolOutput = stdout
	toolResult.OutputFiles = make([]string, len(outputFiles))
	var wg common.WorkGroup
	for i, name := range outputFiles {
		i := i
		name := name
		wg.Spawn(func() {
			var idToWrite string
			id, err := nats2.RobustPubObjectFileRandomName(osb, name)
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
		object, err := nats2.TypedRobustPutObjectRandomName(osb, &toolResult, serializer)
		runResult.ToolResultId = object.Name
		common.HandleErrLog(err, logger)
		if err != nil {
			return err
		}
	}
	_, _, err := kvb.CAS(
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
