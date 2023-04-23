package main

import (
	"exec/cmd"
	"exec/tools"
	"flag"
	"fmt"
	"github.com/nats-io/nats.go"
	"log"
	"math/rand"
	"os"
	"strconv"
)

func getUniqueName() string {
	return strconv.FormatInt(rand.Int63(), 16)
}

func worker(
	sub *nats.Subscription,
	sosb nats.ObjectStore,
	rosb nats.ObjectStore,
	kvb nats.KeyValue,
	logger *log.Logger,
) {
	// TODO
	//var errorCount = 0
	//for {
	//	msgRaw, err := sub.Fetch(1)
	//	if err != nil {
	//		logger.Printf("Error occurred: %+v\n", err)
	//		errorCount++
	//		if errorCount == 10 {
	//			logger.Fatalf("10 errors in a row, there must be something wrong\n")
	//		}
	//		continue
	//	}
	//	if len(msgRaw) != 1 {
	//		logger.Fatalf("Expected batch of size 1 but got %d\n", len(msgRaw))
	//	}
	//	logger.Printf("Received message: %s\n", err)
	//	var msg cmd.TaskMsg
	//	err = json.Unmarshal(msgRaw[0].Data, &msg)
	//	tools.HandlePanic(err)
	//
	//	baseName := getUniqueName()
	//	inputName := "/tmp/" + baseName + ".cpp"
	//	outputName := "/tmp/" + baseName
	//	err = sosb.GetFile(msg.ObjectStoreKey, inputName)
	//	if err != nil {
	//		logger.Printf("Error retrieving the %s object: %+v\n", msg.ObjectStoreKey, err)
	//	}
	//
	//	subProc := exec.Command("g++", inputName, "-o", outputName, "-std=c++2a", "-Wall")
	//
	//	stderr := new(bytes.Buffer)
	//	subProc.Stderr = stderr
	//	tools.HandleErrLog(subProc.Start(), logger)
	//
	//	var status string
	//	select {
	//	case <-time.After(5 * time.Second):
	//		status = "Time limit exceeded"
	//		subProc.Process.Kill()
	//		//case subProc.CombinedOutput()
	//	}
	//
	//	tools.HandleErrLog(os.Remove(inputName), logger)
	//	tools.HandleErrLog(os.Remove(outputName), logger)
	//
	//}
}

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

	js, err := nc.JetStream(nats.PublishAsyncMaxPending(256))
	tools.HandlePanic(err)

	sosb, err := js.ObjectStore(workerConfig.SourceObjectStoreBucketConfig.Name)
	tools.HandlePanic(err)

	rosb, err := js.ObjectStore(workerConfig.ResultObjectStoreBucketConfig.Name)

	kvb, err := js.KeyValue(workerConfig.KeyValueBucketConfig.Name)

	consumerConfig := workerConfig.ConsumerConfig
	sub, err := js.PullSubscribe("", consumerConfig.Name, nats.Bind(consumerConfig.StreamName, consumerConfig.Name))
	tools.HandlePanic(err)

	var wg tools.WorkGroup
	for i := 0; i < workerConfig.WorkerThreads; i++ {
		i := i
		wg.Spawn(func() {
			logger := log.New(
				os.Stderr,
				fmt.Sprintf("Worker #%d: ", i),
				log.LstdFlags|log.LUTC|log.Lmsgprefix|log.Lmicroseconds,
			)
			worker(sub, sosb, rosb, kvb, logger)
		})
	}

	wg.Wait()
}
