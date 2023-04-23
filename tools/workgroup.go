package tools

import "sync"

type WorkGroup struct {
	wg sync.WaitGroup
}

func (wg *WorkGroup) Spawn(job func()) {
	wg.wg.Add(1)
	go func() {
		job()
		wg.wg.Done()
	}()
}

func (wg *WorkGroup) Wait() {
	wg.wg.Wait()
}
