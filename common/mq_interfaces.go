package common

import (
	"context"
	"errors"
)

type Message[T any] interface {
	Content() *T
	Ack() error
	NAck() error
	AckInProgress() error
}

type PullSubscriber[T any] interface {
	Fetch(n int, ctx context.Context) ([]Message[T], error)
}

type Publisher[T any] interface {
	PublishSync(subject string, msg *T) error
}

type KeyValueEntry[T any] interface {
	Key() string
	Value() *T
	Revision() uint64
}

var ErrWrongRevNumber = errors.New("revision number mismatch")

type KeyValueBucket[T any] interface {
	Get(key string) (KeyValueEntry[T], error)
	Create(key string, value *T) (uint64, error)

	// Update on revision number mismatch returns ErrWrongRevNumber
	Update(key string, value *T, last uint64) (uint64, error)
	CAS(
		key string,
		stopPredicate func(*T) (bool, error),
		alter func(*T) error,
	) (*T, uint64, error)
}
