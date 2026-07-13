package queue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WorkerPolicy defines worker limits and scaling behavior.
type WorkerPolicy struct {
	MinWorkers    int
	MaxWorkers    int
	JobsPerWorker int64
	IdleTimeout   time.Duration
}

// workerGroup owns the worker goroutines consuming one queue.
type workerGroup[T any] struct {
	ctx           context.Context
	cancel        context.CancelFunc
	queue         Queue[T]
	processor     ProcessFunc[T]
	panicHandler  PanicHandler[T]
	workerFactory WorkerFactory[T]
	policy        WorkerPolicy

	mu            sync.Mutex
	activeWorkers int
	stopped       bool
}

func newWorkerGroup[T any](
	queue Queue[T],
	processor ProcessFunc[T],
	policy WorkerPolicy,
	panicHandler PanicHandler[T],
	workerFactory WorkerFactory[T],
) *workerGroup[T] {
	// #nosec G118 -- cancel is stored in workerGroup and called by Stop.
	ctx, cancel := context.WithCancel(queue.Context())
	return &workerGroup[T]{
		ctx:           ctx,
		cancel:        cancel,
		queue:         queue,
		processor:     processor,
		panicHandler:  panicHandler,
		workerFactory: workerFactory,
		policy:        policy,
	}
}

// Start launches the minimum number of workers.
func (g *workerGroup[T]) Start() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.stopped {
		return
	}

	for g.activeWorkers < g.policy.MinWorkers {
		g.startWorkerWithoutLock()
	}
}

// NotifyEnqueue scales the group up to the desired number of workers.
func (g *workerGroup[T]) NotifyEnqueue() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.stopped {
		return
	}

	workerCount := g.calculateWorkerCount()
	for g.activeWorkers < workerCount {
		g.startWorkerWithoutLock()
	}
}

// ActiveWorkers returns the number of running workers.
func (g *workerGroup[T]) ActiveWorkers() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.activeWorkers
}

// Stop requests cancellation of all workers.
func (g *workerGroup[T]) Stop() {
	g.mu.Lock()
	if g.stopped {
		g.mu.Unlock()
		return
	}
	g.stopped = true
	g.cancel()
	g.mu.Unlock()
}

func (g *workerGroup[T]) calculateWorkerCount() int {
	queueLen := g.queue.Len()
	workerCount := queueLen / g.policy.JobsPerWorker
	if queueLen%g.policy.JobsPerWorker != 0 {
		workerCount++
	}

	if workerCount < int64(g.policy.MinWorkers) {
		return g.policy.MinWorkers
	}

	if workerCount > int64(g.policy.MaxWorkers) {
		return g.policy.MaxWorkers
	}

	return int(workerCount)
}

func (g *workerGroup[T]) startWorkerWithoutLock() {
	worker := g.workerFactory(WorkerConfig[T]{
		Queue:        g.queue,
		Processor:    g.processor,
		PanicHandler: g.panicHandler,
		IdleTimeout:  g.policy.IdleTimeout,
	})

	g.activeWorkers++
	go g.runWorker(worker)
}

func (g *workerGroup[T]) runWorker(worker Worker) {
	defer func() {
		if recover() != nil {
			g.workerDecrement()
		}
	}()

	for {
		result := worker.Run(g.ctx)

		// остановка воркера или остановка группы
		if result == WorkerStopped || g.ctx.Err() != nil {
			g.workerDecrement()
			return
		}

		// таймаут ожидания воркера
		if g.tryRetireWorker() {
			return
		}

		// Ниже MinWorkers уходить нельзя, запускаем Run снова.
	}
}

func (g *workerGroup[T]) tryRetireWorker() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.stopped || g.activeWorkers <= g.policy.MinWorkers {
		return false
	}

	g.activeWorkers--
	return true
}

func (g *workerGroup[T]) workerDecrement() {
	g.mu.Lock()
	g.activeWorkers--
	g.mu.Unlock()
}

func (p WorkerPolicy) validate() error {
	switch {
	case p.MinWorkers < 1:
		return fmt.Errorf("min workers must be at least 1")
	case p.MaxWorkers < p.MinWorkers:
		return fmt.Errorf("max workers must be greater than or equal to min workers")
	case p.JobsPerWorker < 1:
		return fmt.Errorf("jobs per worker must be at least 1")
	case p.IdleTimeout <= 0:
		return fmt.Errorf("idle timeout must be positive")
	default:
		return nil
	}
}
