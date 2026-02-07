package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Lakshamana/graceful-shutdown-go/worker"
)

const coroutines = 3

func makeCtx(parent context.Context) (context.Context, context.CancelFunc) {
	haltSignals := []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, haltSignals...) // redirect sigChan <- haltSignals...

	ctx, cancel := context.WithCancel(parent)

	go func() {
		defer signal.Stop(sigChan) // this closes sigChan automatically

		select {
		case <-sigChan:
			fmt.Printf("\rSignal received, canceling context\n")
			cancel()
		case <-ctx.Done():
			fmt.Println("Context canceled")
		}
	}()

	return ctx, cancel
}

func startWorker(idx int, ctx context.Context, wg *sync.WaitGroup, lines chan<- struct{}, shutdownBarrier <-chan struct{}) {
	defer wg.Done()

	w := worker.NewWorker(idx)

	go func() {
		if err := w.ListenAndServe(); err != nil {
			fmt.Printf("Worker %d error: %v\n", idx, err)
		}
	}()

	go func() {
		for range w.Recv() {
			lines <- struct{}{}
		}
	}()

	<-ctx.Done()
	<-shutdownBarrier // wait for main to print "waiting" message

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := w.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Worker %d shutdown error: %v\n", idx, err)
	}

	fmt.Printf("worker %d is done\n", idx)
}

func main() {
	ctx, cancel := makeCtx(context.Background())
	defer cancel()

	lines := make(chan struct{})
	var lineCtr atomic.Int32

	shutdownBarrier := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(coroutines)

	for i := range coroutines {
		go startWorker(i, ctx, &wg, lines, shutdownBarrier)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-lines:
				current := lineCtr.Load()
				if current%coroutines == 0 {
					fmt.Println()
				}
				lineCtr.Store(current + 1)
			}
		}
	}()

	<-ctx.Done()
	fmt.Printf("waiting %d coroutines to stop...\n", coroutines)
	close(shutdownBarrier) // signal all workers to proceed with shutdown

	wg.Wait()

	fmt.Println("Master server has shut down")
}
