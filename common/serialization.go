package common

import "encoding/json"

type Serializer[T any] interface {
	Serialize(value *T) ([]byte, error)
	Deserialize(data []byte) (*T, error)
}

type JsonSerializer[T any] struct{}

func (*JsonSerializer[T]) Serialize(value *T) ([]byte, error) {
	return json.Marshal(value)
}

func (*JsonSerializer[T]) Deserialize(data []byte) (*T, error) {
	var ret T
	if err := json.Unmarshal(data, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}
