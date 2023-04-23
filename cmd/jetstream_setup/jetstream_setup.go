package main

import (
	"exec/cmd"
	"exec/tools"
	"flag"
	"github.com/nats-io/nats.go"
	"os"
)

func main() {
	configPath := flag.String("config-file", "worker-config.json", "Path to the worker config file")
	dotEnvPath := flag.String("env-file", ".env", "Path to .env-like file, \"-\" "+
		"or non-existing file for none")
	help := flag.Bool("help", false, "Print help")
	flag.Parse()

	if *help {
		tools.Printfln("Usage: %s", os.Args[0])
		flag.PrintDefaults()
		return
	}

	env := cmd.GetEnv(*dotEnvPath)
	var workerConfig cmd.WorkerConfig
	tools.HandlePanic(cmd.ParseConfigFileWithRespectToEnv(*configPath, env, &workerConfig))

	nc, err := workerConfig.ConnectionConfig.Connect()
	tools.HandlePanic(err)
	defer nc.Close()

	js, err := nc.JetStream()
	tools.HandlePanic(err)

	consumerConfig := workerConfig.ConsumerConfig
	_, err = SetStream(
		js,
		&nats.StreamConfig{
			Name:      consumerConfig.StreamName,
			Subjects:  []string{consumerConfig.StreamName, consumerConfig.StreamName + ".>"},
			Retention: nats.WorkQueuePolicy,
		},
	)
	tools.HandlePanic(err)

	_, err = SetConsumer(
		js,
		consumerConfig.StreamName,
		&nats.ConsumerConfig{
			Name:      consumerConfig.Name,
			Durable:   consumerConfig.Name,
			AckPolicy: nats.AckExplicitPolicy,
			AckWait:   consumerConfig.AckWaitTime.Duration,
		},
	)
	tools.HandlePanic(err)

	sourceBucket := workerConfig.SourceObjectStoreBucketConfig
	_, err = CreateOrGetObjectStoreBucket(
		js,
		&nats.ObjectStoreConfig{
			Bucket:      sourceBucket.Name,
			Description: sourceBucket.Description,
		})
	tools.HandlePanic(err)

	resultBucket := workerConfig.ResultObjectStoreBucketConfig
	_, err = CreateOrGetObjectStoreBucket(
		js,
		&nats.ObjectStoreConfig{
			Bucket:      resultBucket.Name,
			Description: resultBucket.Description,
		})
	tools.HandlePanic(err)
}
