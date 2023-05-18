package nats

import (
	"context"
	"errors"
	"exec/common"
	"github.com/cenkalti/backoff/v4"
	"github.com/nats-io/nats.go"
	"io"
	"os"
	"time"
)

func retryOnPredicate(action func() error, retryPredicate func(error) bool) error {
	back := backoff.NewExponentialBackOff()

	for {
		err := action()
		if retryPredicate(err) {
			time.Sleep(back.NextBackOff())
			continue
		}
		return err
	}

}

func retryOnError(action func() error, errorList []error) error {
	return retryOnPredicate(
		action,
		func(err error) bool {
			for _, e := range errorList {
				if errors.Is(err, e) {
					return true
				}
			}
			return false
		},
	)
}

func RobustPublishSync(js nats.JetStream, subj string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	var ack *nats.PubAck = nil
	err := retryOnError(
		func() error {
			a, err := js.Publish(subj, data, opts...)
			if err == nil {
				ack = a
			}
			return err
		},
		[]error{nats.ErrTimeout},
	)
	return ack, err
}

func RobustFetch(sub *nats.Subscription, n int, ctx context.Context, opts ...nats.PullOpt) ([]*nats.Msg, error) {
	var msgs []*nats.Msg = nil
	return msgs, retryOnError(
		func() error {
			m, err := sub.Fetch(n, append(opts, nats.Context(ctx))...)
			if err == nil {
				msgs = m
			}
			return err
		},
		[]error{nats.ErrConsumerLeadershipChanged, nats.ErrTimeout},
	)
}

func RobustGetObjectFile(osb nats.ObjectStore, id string, filePath string, opts ...nats.GetObjectOpt) error {
	return retryOnError(
		func() error {
			return osb.GetFile(id, filePath, opts...)
		},
		[]error{nats.ErrTimeout},
	)
}

func RobustGetObjectBytes(osb nats.ObjectStore, id string, opts ...nats.GetObjectOpt) ([]byte, error) {
	var data []byte
	return data, retryOnError(
		func() error {
			d, err := osb.GetBytes(id, opts...)
			if err == nil {
				data = d
			}
			return err
		},
		[]error{nats.ErrTimeout},
	)
}

func RobustPutObject(osb nats.ObjectStore, object io.Reader, objectName string, opts ...nats.ObjectOpt) (*nats.ObjectInfo, error) {
	var objectInfo *nats.ObjectInfo
	return objectInfo, retryOnError(
		func() error {
			oi, err := osb.Put(&nats.ObjectMeta{Name: objectName}, object, opts...)
			if err == nil {
				objectInfo = oi
			}
			return err
		},
		[]error{nats.ErrTimeout},
	)

}

func RobustPutObjectFile(osb nats.ObjectStore, filePath string, objectName string, opts ...nats.ObjectOpt) (*nats.ObjectInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return RobustPutObject(osb, file, objectName, opts...)
}

func RobustPubObjectFileRandomName(osb nats.ObjectStore, filePath string, opts ...nats.ObjectOpt) (*nats.ObjectInfo, error) {
	return RobustPutObjectFile(osb, filePath, common.GetRandomId(), opts...)
}

func RobustPutObjectRandomName(osb nats.ObjectStore, object io.Reader, opts ...nats.ObjectOpt) (*nats.ObjectInfo, error) {
	return RobustPutObject(osb, object, common.GetRandomId(), opts...)
}

func RobustGetKVEntry(kvb nats.KeyValue, key string) (nats.KeyValueEntry, error) {
	var result nats.KeyValueEntry
	return result, retryOnError(
		func() error {
			r, err := kvb.Get(key)
			if err == nil {
				result = r
			}
			return err
		},
		[]error{nats.ErrTimeout},
	)
}

func RobustCreateKVEntry(kvb nats.KeyValue, key string, value []byte) (uint64, error) {
	var result uint64
	return result, retryOnError(
		func() error {
			r, err := kvb.Create(key, value)
			if err == nil {
				result = r
			}
			return err
		},
		[]error{nats.ErrTimeout},
	)
}

func RobustUpdateKVEntry(kvb nats.KeyValue, key string, last uint64, newVal []byte) (uint64, error) {
	var result uint64
	return result, retryOnError(
		func() error {
			r, err := kvb.Update(key, newVal, last)
			if err == nil {
				result = r
			}
			return err
		},
		[]error{nats.ErrTimeout},
	)
}
