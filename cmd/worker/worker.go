package main

import (
	"bytes"
	"context"
	"errors"
	"exec/cmd"
	"exec/common"
	"github.com/nats-io/nats.go"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func worker(
	sub common.PullSubscriber[cmd.TaskMsg],
	osb nats.ObjectStore,
	kvb common.KeyValueBucket[cmd.RunResult],
	logger *log.Logger,
	toolsPath string,
	serializer common.Serializer[cmd.ToolResult],
) {
	var errorCount = 0
	for {
		var fileCleanup common.Cleanup
		{
			msgs, err := sub.Fetch(1, context.Background())
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
			content := msg.Content()
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
			err = uploadResultsAndNotify(osb, kvb, outputFiles, stdout.String(), content, logger, serializer)

			if err != nil {
				goto cleanup
			}
			common.HandleErrLog(msg.Ack(), logger)
		}

	cleanup:
		fileCleanup.Do()
	}
}
