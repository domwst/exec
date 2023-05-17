package common

import (
	"errors"
	"github.com/nats-io/nats.go"
)

func CAS[T any](
	kvb nats.KeyValue,
	key string,
	stopPredicate func(*T) (bool, error),
	alter func(*T) error,
) (*T, uint64, error) {
	for {
		entry, err := TypedRobustGetKVEntry[T](kvb, key)
		if err != nil {
			return nil, 0, err
		}
		content := entry.Content
		if b, err := stopPredicate(content); err != nil {
			return nil, 0, err
		} else if b {
			return content, entry.Revision(), nil
		}

		err = alter(content)
		if err != nil {
			return nil, 0, err
		}
		rev, err := TypedRobustUpdateKVEntry(kvb, key, entry.Revision(), content)
		if errors.Is(err, nats.ErrKeyExists) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		return content, rev, nil
	}
}
