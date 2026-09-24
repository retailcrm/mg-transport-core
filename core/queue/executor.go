package queue

import (
	"context"
	"errors"
	"time"
)

// ExecutorInfo is an observability snapshot of a single executor: its queue ID, the last enqueue time,
// the driver statistics, and the number of active workers.
type ExecutorInfo struct {
	ID              int
	LastEnqueueTime time.Time
	Stats           Stats
	ActiveWorkers   int
}

// Executor operates one queue end to end: it owns the Queue, its worker group, and the queue lifecycle.
// Executors are created by a Store through newExecutor and are addressed by their numeric queue ID.
type Executor[T any] struct {
	queue   *Queue[T]
	workers *workerGroup[T]
}

func newExecutor[T any](id int, driver Driver[T], processor Processor[T], policy WorkerPolicy,
	panicHandler PanicHandler[T], unsettled UnsettledProcessor[T], factory WorkerFactory[T],
) *Executor[T] {
	q := New(id, driver)
	executor := &Executor[T]{queue: q}
	executor.workers = newWorkerGroup(q, processor, policy, panicHandler, unsettled, factory)
	executor.workers.Start()
	return executor
}

// ID returns the queue identifier of the underlying queue.
func (e *Executor[T]) ID() int { return e.queue.ID() }

// Enqueue adds an item to the queue and immediately notifies the worker group so scaling can react
// without waiting for the next periodic tick.
func (e *Executor[T]) Enqueue(ctx context.Context, value T, options ...EnqueueOption) error {
	if err := e.queue.Enqueue(ctx, value, options...); err != nil {
		return err
	}
	e.workers.Notify()
	return nil
}

// Info collects the executor observability snapshot, including driver statistics.
func (e *Executor[T]) Info(ctx context.Context) (ExecutorInfo, error) {
	stats, err := e.queue.Stats(ctx)
	return ExecutorInfo{
		ID: e.ID(), LastEnqueueTime: e.queue.LastEnqueueTime(), Stats: stats,
		ActiveWorkers: e.workers.ActiveWorkers(),
	}, err
}

// CloseIntake stops accepting new items while allowing workers to finish the queued ones.
func (e *Executor[T]) CloseIntake() { e.queue.CloseIntake() }

// Drain blocks until the driver reports no queued or in-flight items, or until the context expires.
// Shared durable drivers include other replicas' work; use DrainLocal for one process.
func (e *Executor[T]) Drain(ctx context.Context) error {
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	for {
		stats, err := e.queue.Stats(ctx)
		if err != nil {
			return err
		}
		if stats.Queued() == 0 && stats.InFlight == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// DrainLocal stops this executor from fetching new deliveries and waits for its running workers.
// Queued items remain in the driver for other consumers. Processor contexts stay active until Stop.
func (e *Executor[T]) DrainLocal(ctx context.Context) error {
	e.quiesceLocal()
	return e.workers.Wait(ctx)
}

func (e *Executor[T]) quiesceLocal() {
	e.CloseIntake()
	e.workers.Quiesce()
}

// Close cancels the worker group, closes the queue, and waits for the workers to stop. The returned
// error joins the shutdown error and any worker wait error.
func (e *Executor[T]) Close(ctx context.Context) error {
	closeErr := e.shutdown(ctx)
	return errors.Join(closeErr, e.workers.Wait(ctx))
}

func (e *Executor[T]) shutdown(ctx context.Context) error {
	e.workers.Cancel()
	return e.queue.Close(ctx)
}
