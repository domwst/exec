package nats

import (
	"context"
	"errors"
	"exec/common"
	"github.com/nats-io/nats.go"
)

type jsSubscriptionTypedWrapper[T any] struct {
	sub        *nats.Subscription
	serializer common.Serializer[T]
}

func NewSubscriptionWrapper[T any](sub *nats.Subscription, serializer common.Serializer[T]) common.PullSubscriber[T] {
	return &jsSubscriptionTypedWrapper[T]{
		sub:        sub,
		serializer: serializer,
	}
}

type DeserializedNatsMsg[T any] struct {
	Msg     *nats.Msg
	content *T
}

func (msg *DeserializedNatsMsg[T]) Content() *T {
	return msg.content
}

func (msg *DeserializedNatsMsg[T]) Ack() error {
	return msg.Msg.AckSync()
}

func (msg *DeserializedNatsMsg[T]) NAck() error {
	return msg.Msg.Nak()
}

func (msg *DeserializedNatsMsg[T]) AckInProgress() error {
	return msg.Msg.InProgress()
}

func (js *jsSubscriptionTypedWrapper[T]) Fetch(n int, ctx context.Context) ([]common.Message[T], error) {
	msgs, err := RobustFetch(js.sub, n, ctx)
	if err != nil {
		return nil, err
	}
	ret := make([]common.Message[T], len(msgs))
	for i, msg := range msgs {
		content, err := js.serializer.Deserialize(msg.Data)
		if err != nil {
			return nil, err
		}
		ret[i] = &DeserializedNatsMsg[T]{
			Msg:     msg,
			content: content,
		}
	}
	return ret, nil
}

type jsPublisherTypedWrapper[T any] struct {
	js         nats.JetStream
	serializer common.Serializer[T]
}

func NewPublisherWrapper[T any](js nats.JetStream, serializer common.Serializer[T]) common.Publisher[T] {
	return &jsPublisherTypedWrapper[T]{
		js:         js,
		serializer: serializer,
	}
}

func (js *jsPublisherTypedWrapper[T]) PublishSync(subject string, v *T) error {
	data, err := js.serializer.Serialize(v)
	if err != nil {
		return err
	}
	_, err = RobustPublishSync(js.js, subject, data)
	return err
}

type keyValueTypedWrapper[T any] struct {
	kv         nats.KeyValue
	serializer common.Serializer[T]
}

func NewKeyValueTypedWrapper[T any](kv nats.KeyValue, serializer common.Serializer[T]) common.KeyValueBucket[T] {
	return &keyValueTypedWrapper[T]{
		kv:         kv,
		serializer: serializer,
	}
}

type DeserializedKVEntry[T any] struct {
	entry   nats.KeyValueEntry
	content *T
}

func (e *DeserializedKVEntry[T]) Key() string {
	return e.entry.Key()
}

func (e *DeserializedKVEntry[T]) Value() *T {
	return e.content
}

func (e *DeserializedKVEntry[T]) Revision() uint64 {
	return e.entry.Revision()
}

func (kv *keyValueTypedWrapper[T]) Get(key string) (common.KeyValueEntry[T], error) {
	entry, err := RobustGetKVEntry(kv.kv, key)
	if err != nil {
		return nil, err
	}
	content, err := kv.serializer.Deserialize(entry.Value())
	if err != nil {
		return nil, err
	}
	return &DeserializedKVEntry[T]{
		entry:   entry,
		content: content,
	}, nil
}

func (kv *keyValueTypedWrapper[T]) Create(key string, value *T) (uint64, error) {
	data, err := kv.serializer.Serialize(value)
	if err != nil {
		return 0, err
	}
	return RobustCreateKVEntry(kv.kv, key, data)
}

func (kv *keyValueTypedWrapper[T]) Update(key string, value *T, last uint64) (uint64, error) {
	data, err := kv.serializer.Serialize(value)
	if errors.Is(err, nats.ErrKeyExists) {
		return 0, common.ErrWrongRevNumber
	}
	if err != nil {
		return 0, err
	}
	return RobustUpdateKVEntry(kv.kv, key, last, data)
}

func (kv *keyValueTypedWrapper[T]) CAS(
	key string,
	stopPredicate func(*T) (bool, error),
	alter func(*T) error,
) (*T, uint64, error) {
	for {
		entry, err := kv.Get(key)
		if err != nil {
			return nil, 0, err
		}
		stop, err := stopPredicate(entry.Value())
		if err != nil {
			return nil, 0, err
		}
		if stop {
			return entry.Value(), entry.Revision(), nil
		}
		value := entry.Value()
		err = alter(value)
		if err != nil {
			return nil, 0, err
		}
		newRev, err := kv.Update(key, value, entry.Revision())
		if errors.Is(err, common.ErrWrongRevNumber) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		return value, newRev, nil
	}
}
