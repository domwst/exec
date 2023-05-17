package common

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/nats-io/nats.go"
)

func RobustTypedPublishSync[T any](js nats.JetStreamContext, subj string, content *T, opts ...nats.PubOpt) (*nats.PubAck, error) {
	data, err := Serialize(content)
	if err != nil {
		return nil, err
	}
	return RobustPublishSync(js, subj, data, opts...)
}

type DeserializedNatsMsg[T any] struct {
	*nats.Msg
	Content *T
}

func TypedRobustFetch[T any](
	sub *nats.Subscription,
	n int,
	ctx context.Context,
	opts ...nats.PullOpt,
) ([]*DeserializedNatsMsg[T], error) {
	msgs, err := RobustFetch(sub, n, ctx, opts...)
	if err != nil {
		return nil, err
	}
	ret := make([]*DeserializedNatsMsg[T], len(msgs))
	for i, msg := range msgs {
		ret[i].Msg = msg
		ret[i].Content, err = Deserialize[T](msg.Data)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func TypedRobustGetObject[T any](
	osb nats.ObjectStore,
	id string,
	opts ...nats.GetObjectOpt,
) (*T, error) {
	data, err := RobustGetObjectBytes(osb, id, opts...)
	if err != nil {
		return nil, err
	}
	var ret T
	err = json.Unmarshal(data, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func TypedRobustPutObject[T any](
	osb nats.ObjectStore,
	content *T,
	objectName string,
	opts ...nats.ObjectOpt,
) (*nats.ObjectInfo, error) {
	data, err := Serialize(content)
	if err != nil {
		return nil, err
	}
	return RobustPutObject(osb, bytes.NewReader(data), objectName, opts...)
}

func TypedRobustPutObjectRandomName[T any](
	osb nats.ObjectStore,
	content *T,
	opts ...nats.ObjectOpt,
) (*nats.ObjectInfo, error) {
	data, err := Serialize(content)
	if err != nil {
		return nil, err
	}
	return RobustPutObjectRandomName(osb, bytes.NewReader(data), opts...)
}

type DeserializedKVEntry[T any] struct {
	nats.KeyValueEntry
	Content *T
}

func TypedRobustGetKVEntry[T any](
	kvb nats.KeyValue,
	key string,
) (*DeserializedKVEntry[T], error) {
	entry, err := RobustGetKVEntry(kvb, key)
	if err != nil {
		return nil, err
	}
	content, err := Deserialize[T](entry.Value())
	if err != nil {
		return nil, err
	}
	return &DeserializedKVEntry[T]{
		KeyValueEntry: entry,
		Content:       content,
	}, nil
}

func TypedRobustCreateKVEntry[T any](
	kvb nats.KeyValue,
	key string,
	value *T,
) (uint64, error) {
	data, err := Serialize(value)
	if err != nil {
		return 0, err
	}
	return RobustCreateKVEntry(kvb, key, data)
}

func TypedRobustUpdateKVEntry[T any](kvb nats.KeyValue, key string, last uint64, newVal *T) (uint64, error) {
	data, err := Serialize(newVal)
	if err != nil {
		return 0, err
	}
	return RobustUpdateKVEntry(kvb, key, last, data)
}
