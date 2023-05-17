package common

import "encoding/json"

func Serialize[T any](v *T) ([]byte, error) {
	return json.Marshal(v)
}

func Deserialize[T any](data []byte) (*T, error) {
	var ret T
	if err := json.Unmarshal(data, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}
