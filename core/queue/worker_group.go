package queue

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ScaleInfo is the input for scaling decisions: queue identity, the last enqueue time, the driver
// statistics, and the number of currently active workers.
type ScaleInfo struct {
	ID              int
	LastEnqueueTime time.Time
	Stats           Stats
	ActiveWorkers   int
}

// DesiredWorkersFunc computes the desired worker count for a queue. It is consulted by the worker
// group controller on every scaling tick and on enqueue notifications. Returned values are clamped to
// the [MinWorkers, MaxWorkers] range; panics are contained and fall back to MinWorkers.
type DesiredWorkersFunc func(ScaleInfo) int

// WorkerPolicy describes how the worker pool of a single executor scales and how workers behave:
//
//   - MinWorkers/MaxWorkers bound the pool size; at least one worker must be allowed.
//   - JobsPerWorker sets the default scaling ratio: ceil(Ready/JobsPerWorker) workers are desired.
//   - DesiredWorkers, when set, replaces the JobsPerWorker ratio with a custom function.
//   - IdleTimeout bounds a dequeue attempt before a worker reports idleness and becomes retirable.
//   - ScaleInterval is the period of the periodic scaling tick; scaling also runs on enqueue.
//   - RestartDelay throttles worker replacement after an unexpected worker failure.
type WorkerPolicy struct {
	MinWorkers     int
	MaxWorkers     int
	JobsPerWorker  int64
	IdleTimeout    time.Duration
	ScaleInterval  time.Duration
	RestartDelay   time.Duration
	DesiredWorkers DesiredWorkersFunc
}

func (p WorkerPolicy) validate() error {
	switch {
	case p.MinWorkers < 0:
		return errors.New("min workers cannot be negative")
	case p.MaxWorkers < 1:
		return errors.New("max workers must be at least 1")
	case p.MaxWorkers < p.MinWorkers:
		return errors.New("max workers must be greater than or equal to min workers")
	case p.DesiredWorkers == nil && p.JobsPerWorker < 1:
		return errors.New("jobs per worker must be at least 1")
	case p.IdleTimeout <= 0:
		return errors.New("idle timeout must be positive")
	case p.ScaleInterval <= 0:
		return errors.New("scale interval must be positive")
	case p.RestartDelay < 0:
		return errors.New("restart delay cannot be negative")
	default:
		return nil
	}
}

type workerGroup[T any] struct {
	ctx           context.Context
	cancel        context.CancelFunc
	controlCtx    context.Context
	stopControl   context.CancelFunc
	queue         *Queue[T]
	processor     Processor[T]
	panicHandler  PanicHandler[T]
	unsettled     UnsettledProcessor[T]
	workerFactory WorkerFactory[T]
	policy        WorkerPolicy
	notify        chan struct{}

	mu            sync.Mutex
	activeWorkers int
	desired       int
	stopped       bool
	wg            sync.WaitGroup
}

func newWorkerGroup[T any](queue *Queue[T], processor Processor[T], policy WorkerPolicy,
	panicHandler PanicHandler[T], unsettled UnsettledProcessor[T], factory WorkerFactory[T],
) *workerGroup[T] {
	ctx, cancel := context.WithCancel(queue.Context())
	controlCtx, stopControl := context.WithCancel(ctx)
	return &workerGroup[T]{
		ctx: ctx, cancel: cancel, queue: queue, processor: processor, policy: policy,
		controlCtx: controlCtx, stopControl: stopControl,
		panicHandler: panicHandler, unsettled: unsettled, workerFactory: factory,
		notify: make(chan struct{}, 1), desired: policy.MinWorkers,
	}
}

func (g *workerGroup[T]) Start() {
	g.mu.Lock()
	for g.activeWorkers < g.policy.MinWorkers {
		g.startWorkerLocked()
	}
	g.wg.Go(g.control)
	g.mu.Unlock()
	g.Notify()
}

func (g *workerGroup[T]) Notify() {
	select {
	case g.notify <- struct{}{}:
	default:
	}
}

func (g *workerGroup[T]) control() {
	ticker := time.NewTicker(g.policy.ScaleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-g.controlCtx.Done():
			return
		case <-g.notify:
			g.scale()
		case <-ticker.C:
			g.scale()
		}
	}
}

func (g *workerGroup[T]) scale() {
	stats, err := g.queue.Stats(g.controlCtx)
	if err != nil {
		g.mu.Lock()
		if !g.stopped {
			g.desired = g.policy.MinWorkers
			for g.activeWorkers < g.desired {
				g.startWorkerLocked()
			}
		}
		g.mu.Unlock()
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped {
		return
	}
	info := ScaleInfo{ID: g.queue.ID(), LastEnqueueTime: g.queue.LastEnqueueTime(), Stats: stats, ActiveWorkers: g.activeWorkers}
	g.desired = g.workerCount(info)
	for g.activeWorkers < g.desired {
		g.startWorkerLocked()
	}
}

func (g *workerGroup[T]) workerCount(info ScaleInfo) int {
	count := 0
	if g.policy.DesiredWorkers != nil {
		count = invokeDesiredWorkers(g.policy.DesiredWorkers, info, g.policy.MinWorkers)
	} else if info.Stats.Ready > 0 {
		count = int((info.Stats.Ready + g.policy.JobsPerWorker - 1) / g.policy.JobsPerWorker)
	}
	return min(g.policy.MaxWorkers, max(g.policy.MinWorkers, count))
}

func invokeDesiredWorkers(function DesiredWorkersFunc, info ScaleInfo, fallback int) (result int) {
	result = fallback
	defer func() { _ = recover() }()
	return function(info)
}

func (g *workerGroup[T]) startWorkerLocked() {
	worker := g.workerFactory(WorkerConfig[T]{
		Queue: g.queue, Processor: g.processor, PanicHandler: g.panicHandler,
		UnsettledProcessor: g.unsettled, IdleTimeout: g.policy.IdleTimeout,
	})
	g.activeWorkers++
	g.wg.Go(func() { g.runWorker(worker) })
}

func (g *workerGroup[T]) runWorker(worker Worker) {
	restart := false
	defer func() {
		if recover() != nil {
			restart = true
		}
		g.mu.Lock()
		g.activeWorkers--
		stopped := g.stopped
		g.mu.Unlock()
		if restart && !stopped {
			g.notifyAfterRestartDelay()
		}
	}()

	for {
		result := worker.Run(g.ctx)
		if result == WorkerIdle {
			g.mu.Lock()
			retire := g.stopped || g.activeWorkers > max(g.policy.MinWorkers, g.desired)
			g.mu.Unlock()
			if retire {
				return
			}
			continue
		}
		if g.ctx.Err() == nil {
			restart = true
		}
		return
	}
}

func (g *workerGroup[T]) notifyAfterRestartDelay() {
	g.wg.Go(func() {
		timer := time.NewTimer(g.policy.RestartDelay)
		defer timer.Stop()
		select {
		case <-g.controlCtx.Done():
		case <-timer.C:
			g.Notify()
		}
	})
}

func (g *workerGroup[T]) ActiveWorkers() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.activeWorkers
}

func (g *workerGroup[T]) Quiesce() {
	g.mu.Lock()
	if !g.stopped {
		g.stopped = true
		g.stopControl()
		g.queue.stopLocalDequeue()
	}
	g.mu.Unlock()
}

func (g *workerGroup[T]) Cancel() {
	g.Quiesce()
	g.cancel()
}

func (g *workerGroup[T]) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
