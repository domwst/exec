package tools

import (
	"github.com/nats-io/nats.go"
	"reflect"
)

// SetStream Idempotent stream setup
func SetStream(js nats.JetStreamManager, config *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	info, err := js.StreamInfo(config.Name)

	if err != nats.ErrStreamNotFound && err != nil {
		return nil, err
	}

	if info == nil && err == nil {
		panic("both info and err are nil")
	}

	if err == nil && reflect.DeepEqual(info.Config, *config) {
		return info, nil
	}

	var configSetter func(config *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	if err == nats.ErrStreamNotFound {
		configSetter = js.AddStream
	} else {
		configSetter = js.UpdateStream
	}

	return configSetter(config, opts...)
}

func SetConsumer(js nats.JetStreamManager, stream string, config *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	currentConsumerInfo, err := js.ConsumerInfo(stream, config.Name)
	if err != nil && err != nats.ErrConsumerNotFound {
		return nil, err
	}

	if currentConsumerInfo == nil && err == nil {
		panic("both currentConsumerInfo and err are nil")
	}

	if currentConsumerInfo != nil && reflect.DeepEqual(currentConsumerInfo.Config, *config) {
		return currentConsumerInfo, nil
	}

	var configSetter func(stream string, config *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
	if err == nats.ErrConsumerNotFound {
		configSetter = js.AddConsumer
	} else {
		configSetter = js.UpdateConsumer
	}
	return configSetter(stream, config, opts...)
}
