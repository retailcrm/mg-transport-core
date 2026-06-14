package queue

import (
	"context"
	"errors"
	"time"
)

// ProcessFunc processes one item dequeued by a worker.
type ProcessFunc[T any] func(context.Context, T)

// PanicHandler handles a panic raised by ProcessFunc.
type PanicHandler[T any] func(context.Context, T, any)

// WorkerResult describes why a worker returned control to its group.
type WorkerResult uint8

const (
	// WorkerIdle means the worker reached its idle timeout.
	WorkerIdle WorkerResult = iota
	// WorkerStopped means the worker cannot continue.
	WorkerStopped
)

// Worker consumes and processes queue items until it becomes idle or is stopped.
type Worker interface {
	Run(context.Context) WorkerResult
}

// WorkerConfig contains dependencies available to a worker.
type WorkerConfig[T any] struct {
	Queue        Queue[T]
	Processor    ProcessFunc[T]
	PanicHandler PanicHandler[T]
	IdleTimeout  time.Duration
}

// WorkerFactory constructs a worker for a queue.
type WorkerFactory[T any] func(WorkerConfig[T]) Worker

type defaultWorker[T any] struct {
	config WorkerConfig[T]
}

func defaultWorkerFactory[T any](config WorkerConfig[T]) Worker {
	return &defaultWorker[T]{config: config}
}

func (w *defaultWorker[T]) Run(ctx context.Context) WorkerResult {
	for {
		item, err := w.dequeue(ctx)

		if errors.Is(err, context.DeadlineExceeded) {
			return WorkerIdle
		}

		if err != nil {
			return WorkerStopped
		}

		w.process(ctx, item)
	}
}

func (w *defaultWorker[T]) dequeue(ctx context.Context) (T, error) {
	dequeueCtx, cancel := context.WithTimeout(ctx, w.config.IdleTimeout)
	defer cancel()

	return w.config.Queue.DequeueContext(dequeueCtx)
}

func (w *defaultWorker[T]) process(ctx context.Context, item T) {
	defer func() {
		if recovered := recover(); recovered != nil && w.config.PanicHandler != nil {
			w.config.PanicHandler(ctx, item, recovered)
		}
	}()

	w.config.Processor(ctx, item)
}
