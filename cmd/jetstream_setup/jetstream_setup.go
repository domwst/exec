package main

import (
	"exec/cmd"
	"exec/common"
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

	consumerConfig := workerConfig.ConsumerConfig
	_, err = cmd.SetStream(
		js,
		&nats.StreamConfig{
			Name:      consumerConfig.StreamName,
			Subjects:  []string{consumerConfig.StreamName, consumerConfig.StreamName + ".>"},
			Retention: nats.WorkQueuePolicy,
		},
	)
	common.HandlePanic(err)

	_, err = cmd.SetConsumer(
		js,
		consumerConfig.StreamName,
		&nats.ConsumerConfig{
			Name:      consumerConfig.Name,
			Durable:   consumerConfig.Name,
			AckPolicy: nats.AckExplicitPolicy,
			AckWait:   consumerConfig.AckWaitTime.Duration,
		},
	)
	common.HandlePanic(err)

	sourceBucket := workerConfig.ObjectStoreBucketConfig
	_, err = cmd.CreateOrGetObjectStoreBucket(
		js,
		&nats.ObjectStoreConfig{
			Bucket:      sourceBucket.Name,
			Description: sourceBucket.Description,
			Replicas:    sourceBucket.Replicas,
		})
	common.HandlePanic(err)
}
