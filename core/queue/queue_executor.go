package queue

import "context"

// ExecutorInfo describes the current state of a queue executor.
type ExecutorInfo struct {
	Queue         Info
	ActiveWorkers int
}

// Executor owns one queue and its worker group.
type Executor[T any] struct {
	queue     Queue[T]
	workers   *workerGroup[T]
	stopQueue context.CancelFunc
}

func newQueueExecutor[T any](
	id int,
	constructor Constructor[T],
	processor ProcessFunc[T],
	policy WorkerPolicy,
	panicHandler PanicHandler[T],
	workerFactory WorkerFactory[T],
) *Executor[T] {
	q, stopQueue := constructor(id)
	workers := newWorkerGroup(q, processor, policy, panicHandler, workerFactory)
	executor := &Executor[T]{
		queue:     q,
		workers:   workers,
		stopQueue: stopQueue,
	}
	workers.Start()

	return executor
}

// Enqueue adds an item to the queue and scales the worker group if needed.
func (e *Executor[T]) Enqueue(item T) error {
	if err := e.queue.Enqueue(item); err != nil {
		return err
	}

	e.workers.NotifyEnqueue()
	return nil
}

// Info returns queue and worker information.
func (e *Executor[T]) Info() ExecutorInfo {
	return ExecutorInfo{
		Queue:         e.queue,
		ActiveWorkers: e.workers.ActiveWorkers(),
	}
}

func (e *Executor[T]) stop() {
	e.workers.Stop()
	if e.stopQueue != nil {
		e.stopQueue()
	}
}
