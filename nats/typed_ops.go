package nats

import (
	"bytes"
	"exec/common"
	"github.com/nats-io/nats.go"
)

func TypedRobustGetObject[T any](
	osb nats.ObjectStore,
	id string,
	serializer common.Serializer[T],
	opts ...nats.GetObjectOpt,
) (*T, error) {
	data, err := RobustGetObjectBytes(osb, id, opts...)
	if err != nil {
		return nil, err
	}
	ret, err := serializer.Deserialize(data)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func TypedRobustPutObject[T any](
	osb nats.ObjectStore,
	content *T,
	serializer common.Serializer[T],
	objectName string,
	opts ...nats.ObjectOpt,
) (*nats.ObjectInfo, error) {
	data, err := serializer.Serialize(content)
	if err != nil {
		return nil, err
	}
	return RobustPutObject(osb, bytes.NewReader(data), objectName, opts...)
}

func TypedRobustPutObjectRandomName[T any](
	osb nats.ObjectStore,
	content *T,
	serializer common.Serializer[T],
	opts ...nats.ObjectOpt,
) (*nats.ObjectInfo, error) {
	data, err := serializer.Serialize(content)
	if err != nil {
		return nil, err
	}
	return RobustPutObjectRandomName(osb, bytes.NewReader(data), opts...)
}
