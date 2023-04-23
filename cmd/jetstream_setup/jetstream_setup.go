package main

import (
	"exec/common"
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

	env := common.GetEnv(*dotEnvPath)
	var workerConfig common.WorkerConfig
	tools.HandlePanic(common.ParseConfigFileWithRespectToEnv(*configPath, env, &workerConfig))
	tools.Printfln("WorkerConfig: %+v", workerConfig)
	connectionConfig := workerConfig.ConnectionConfig
	nc, err := nats.Connect(
		connectionConfig.NatsUrl,
		nats.UserInfo(connectionConfig.User, connectionConfig.Password),
	)
	tools.HandlePanic(err)
	defer nc.Close()

	js, err := nc.JetStream()
	tools.HandlePanic(err)

	consumerConfig := workerConfig.ConsumerConfig
	_, err = tools.SetStream(
		js,
		&nats.StreamConfig{
			Name:      consumerConfig.StreamName,
			Subjects:  []string{consumerConfig.StreamName, consumerConfig.StreamName + ".>"},
			Retention: nats.WorkQueuePolicy,
		},
	)
	tools.HandlePanic(err)

	_, err = tools.SetConsumer(
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
}
