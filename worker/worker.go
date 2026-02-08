// Package worker exposes a basic worker server implementation
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Worker struct {
	id         int
	closeOnce  sync.Once
	cancelChan chan struct{}
	ping       chan struct{}
	doneChan   chan struct{}
}

func NewWorker(id int) *Worker {
	return &Worker{
		id:         id,
		cancelChan: make(chan struct{}),
		doneChan:   make(chan struct{}),
		ping:       make(chan struct{}),
	}
}

func (w *Worker) ListenAndServe() error {
	defer close(w.doneChan)

	for {
		select {
		case <-w.cancelChan:
			time.Sleep(5 * time.Second)  // induce shutdown to fail by context canceled
			return nil
		default:
			fmt.Printf(">> worker %d doing work...\n", w.id)
			w.send()
			time.Sleep(1 * time.Second)
		}
	}
}

func (w *Worker) send() {
	w.ping <- struct{}{}
}

func (w *Worker) Recv() <-chan struct{} {
	return w.ping
}

func (w *Worker) Shutdown(ctx context.Context) error {
	fmt.Printf(">> worker %d is shutting down...\n", w.id)

	w.closeOnce.Do(func() {
		close(w.cancelChan)
	})

	select {
	case <-w.doneChan:
		fmt.Printf(">> done before time is out: %d\n", w.id)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("Shutdown timeout: %w", ctx.Err())
	}
}
