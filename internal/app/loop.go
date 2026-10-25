package app

import (
	"context"
	"sync"
)

// loop runs every state change on one goroutine - other goroutines only post to it
type loop struct {
	mu    sync.Mutex
	queue []func()
	wake  chan struct{}
}

func newLoop() *loop {
	return &loop{wake: make(chan struct{}, 1)}
}

func (l *loop) post(f func()) {
	l.mu.Lock()
	l.queue = append(l.queue, f)
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *loop) drain() int {
	l.mu.Lock()
	q := l.queue
	l.queue = nil
	l.mu.Unlock()
	for _, f := range q {
		f()
	}
	return len(q)
}

// run handles posted work until done reports true - settled runs whenever the queue empties
func (l *loop) run(ctx context.Context, done func() bool, settled func()) error {
	for {
		for l.drain() > 0 {
		}
		if settled != nil {
			settled()
		}
		if done() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-l.wake:
		}
	}
}
