package tools

import "sync"

type Mutexed[T any] struct {
	mu  sync.Mutex
	val T
}

func CreateMutexed[T any](value T) Mutexed[T] {
	return Mutexed[T]{
		val: value,
	}
}

func (m *Mutexed[T]) Modify(modification func(*T)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	modification(&m.val)
}

func (m *Mutexed[T]) Set(v T) {
	m.Modify(func(p *T) {
		*p = v
	})
}

func (m *Mutexed[T]) Get() (result T) {
	m.Modify(func(p *T) {
		result = *p
	})
	return
}
