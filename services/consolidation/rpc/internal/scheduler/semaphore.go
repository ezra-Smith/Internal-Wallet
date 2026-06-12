package scheduler

import (
	"context"
	"sync"
)

type Semaphore struct {
	ch chan struct{}
}

func NewSemaphore(size int) *Semaphore {
	if size <= 0 {
		size = 1
	}
	return &Semaphore{ch: make(chan struct{}, size)}
}

func (s *Semaphore) Acquire(ctx context.Context) (release func(), ok bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case s.ch <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				select {
				case <-s.ch:
				default:
					panic("semaphore release without matching acquire")
				}
			})
		}, true
	case <-ctx.Done():
		return func() {}, false
	}
}
