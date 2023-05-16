package common

type Pair[T any, U any] struct {
	First  T `json:"first,omitempty"`
	Second U `json:"second,omitempty"`
}
