package queue

import (
	"context"
	"errors"
)

type (
	// Worker represents function which dequeues an item from provided queue and does something with it.
	// Useful when NewWorker implementation isn't agile enough.
	Worker[T any]       func(Queue[T])
	contextQueue[T any] interface {
		DequeueContext(context.Context) (T, error)
	}
	// Processor accepts incoming job and does something with it.
	Processor[T any] func(T, Queue[T])
	// RecoverFunc handles output value received from recover() call.
	RecoverFunc[T any] func(context.Context, T, any)
)

// NewWorker constructs new worker that will retry the given processor until it succeeds
// or is interrupted by the context cancellation. `recover()` value in cause of panics is handled by provided recoverFn.
func NewWorker[T any](ctx context.Context, processor Processor[T], recoverFn RecoverFunc[T], cancelCallbacks ...func()) Worker[T] {
	return func(q Queue[T]) {
		callCancelCallbacks := func() {
			for _, cb := range cancelCallbacks {
				cb()
			}
		}
		dequeue := q.Dequeue
		if contextQueue, ok := q.(contextQueue[T]); ok {
			dequeue = func() (T, error) {
				return contextQueue.DequeueContext(ctx)
			}
		}

		for {
			if ctx.Err() != nil {
				callCancelCallbacks()
				return
			}

			job, err := dequeue()
			if err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					callCancelCallbacks()
				}
				return
			}

			(func() {
				defer func() {
					if r := recover(); r != nil {
						recoverFn(q.Context(), job, r)
					}
				}()
				processor(job, q)
			})()
		}
	}
}

// DummyWorker worker constructor. Returns worker that does nothing.
func DummyWorker[T any]() WorkerConstructor[T] {
	return func(_ int) (Worker[T], context.CancelFunc) {
		return func(_ Queue[T]) {}, func() {}
	}
}

// DummyProcessor does nothing with provided data.
func DummyProcessor[T any](_ T, _ Queue[T]) {}

// RecoverFuncDummy doesn't do anything with the result of `recover()` call.
func RecoverFuncDummy[T any](_ context.Context, _ T, _ any) {}

// Compile-time checks for interface compatibility.
var (
	_ = Processor[int](DummyProcessor[int])
	_ = RecoverFunc[int](RecoverFuncDummy[int])
)
