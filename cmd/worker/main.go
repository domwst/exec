package main

import (
	"exec/cmd"
	"exec/common"
	nats2 "exec/nats"
	"flag"
	"fmt"
	"github.com/nats-io/nats.go"
	"log"
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

	osb, err := js.ObjectStore(workerConfig.ObjectStoreBucketConfig.Name)
	common.HandlePanic(err)

	kvb, err := js.KeyValue(workerConfig.KeyValueBucketConfig.Name)
	common.HandlePanic(err)

	typedKVB := nats2.NewKeyValueTypedWrapper[cmd.RunResult](kvb, &common.JsonSerializer[cmd.RunResult]{})

	consumerConfig := workerConfig.ConsumerConfig
	sub, err := js.PullSubscribe("", consumerConfig.Name, nats.Bind(consumerConfig.StreamName, consumerConfig.Name))
	common.HandlePanic(err)

	typedSub := nats2.NewSubscriptionWrapper[cmd.TaskMsg](sub, &common.JsonSerializer[cmd.TaskMsg]{})

	var wg common.WorkGroup
	for i := 0; i < workerConfig.WorkerThreads; i++ {
		i := i
		wg.Spawn(func() {
			logger := log.New(
				os.Stderr,
				fmt.Sprintf("Worker #%d: ", i),
				log.LstdFlags|log.LUTC|log.Lmsgprefix|log.Lmicroseconds,
			)
			worker(typedSub, osb, typedKVB, logger, workerConfig.PathToTools, &common.JsonSerializer[cmd.ToolResult]{})
		})
	}

	wg.Wait()
}
