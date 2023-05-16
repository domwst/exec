package common

import (
	"errors"
	"github.com/nats-io/nats.go"
)

func CAS(kvb nats.KeyValue, key string, stopPredicate func([]byte) bool, alterRule func([]byte) ([]byte, error)) ([]byte, uint64, error) {
	for {
		entry, err := RobustGetKVEntry(kvb, key)
		if err != nil {
			return nil, 0, err
		}
		if stopPredicate(entry.Value()) {
			return entry.Value(), entry.Revision(), err
		}

		newValue, err := alterRule(entry.Value())
		if err != nil {
			return nil, 0, err
		}
		rev, err := RobustUpdateKVEntry(kvb, key, entry.Revision(), newValue)
		if errors.Is(err, nats.ErrKeyExists) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		return newValue, rev, nil
	}
}
