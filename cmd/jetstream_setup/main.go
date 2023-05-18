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

	consumerConfig := workerConfig.ConsumerConfig

	_, err = cmd.SetStream(
		js,
		&nats.StreamConfig{
			Name:      consumerConfig.StreamName,
			Subjects:  []string{consumerConfig.StreamName, consumerConfig.StreamName + ".>"},
			Retention: nats.WorkQueuePolicy,
			Replicas:  consumerConfig.Replicas,
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
			Replicas:  consumerConfig.Replicas,
		},
	)
	common.HandlePanic(err)

	osBucketConfig := workerConfig.ObjectStoreBucketConfig
	_, err = cmd.CreateOrGetObjectStoreBucket(
		js,
		&nats.ObjectStoreConfig{
			Bucket:      osBucketConfig.Name,
			Description: osBucketConfig.Description,
			Replicas:    osBucketConfig.Replicas,
		})
	common.HandlePanic(err)

	kvBucketConfig := workerConfig.KeyValueBucketConfig
	_, err = cmd.CreateOrGetKeyValueStoreBucket(
		js,
		&nats.KeyValueConfig{
			Bucket:      kvBucketConfig.Name,
			Description: kvBucketConfig.Description,
			Replicas:    kvBucketConfig.Replicas,
		},
	)
}
