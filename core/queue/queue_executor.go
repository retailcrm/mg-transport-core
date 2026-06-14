package queue

import "context"

// QueueExecutorInfo describes the current state of a queue executor.
type QueueExecutorInfo struct {
	Queue         Info
	ActiveWorkers int
}

// QueueExecutor owns one queue and its worker group.
type QueueExecutor[T any] struct {
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
) *QueueExecutor[T] {
	q, stopQueue := constructor(id)
	workers := newWorkerGroup(q, processor, policy, panicHandler)
	executor := &QueueExecutor[T]{
		queue:     q,
		workers:   workers,
		stopQueue: stopQueue,
	}
	workers.Start()

	return executor
}

// Enqueue adds an item to the queue and scales the worker group if needed.
func (e *QueueExecutor[T]) Enqueue(item T) error {
	if err := e.queue.Enqueue(item); err != nil {
		return err
	}

	e.workers.NotifyEnqueue()
	return nil
}

// Info returns queue and worker information.
func (e *QueueExecutor[T]) Info() QueueExecutorInfo {
	return QueueExecutorInfo{
		Queue:         e.queue,
		ActiveWorkers: e.workers.ActiveWorkers(),
	}
}

func (e *QueueExecutor[T]) stop() {
	e.workers.Stop()
	if e.stopQueue != nil {
		e.stopQueue()
	}
}
