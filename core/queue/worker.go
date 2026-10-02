package queue

import (
	"context"
	"errors"
	"time"
)

// Processor consumes a single envelope. It receives the queue ID alongside the envelope so one
// processor can serve every executor of a Store. The processor must settle the envelope with Ack,
// Requeue, or Reject; if it does not (and no UnsettledProcessor is registered), the envelope stays
// pending in the driver.
type Processor[T any] func(context.Context, int, JobEnvelope[T])

// PanicHandler observes the recovered value when a Processor or UnsettledProcessor panics. The panic
// is already contained by the worker; the handler is only a reporting hook.
type PanicHandler[T any] func(context.Context, int, JobEnvelope[T], any)

// UnsettledKind describes why an UnsettledProcessor was invoked.
type UnsettledKind uint8

const (
	// UnsettledReturned means the processor returned without settling the envelope.
	UnsettledReturned UnsettledKind = iota + 1
	// UnsettledPanicked means the processor panicked; the recovered value is in UnsettledCause.Panic.
	UnsettledPanicked
)

// UnsettledCause carries the reason an UnsettledProcessor was invoked.
type UnsettledCause struct {
	Kind  UnsettledKind
	Panic any
}

// UnsettledProcessor handles envelopes that reached the end of processing without an explicit Ack,
// Requeue, or Reject. It is the recommended place for fallback settlement (for example, Reject with
// logging) and for recording envelope losses caused by processor panics.
type UnsettledProcessor[T any] func(context.Context, int, JobEnvelope[T], UnsettledCause)

// WorkerResult reports why a Worker Run call returned.
type WorkerResult uint8

const (
	// WorkerIdle means no envelope arrived within the idle timeout and the worker can be retired.
	WorkerIdle WorkerResult = iota
	// WorkerStopped means the worker hit an error or cancellation and cannot continue.
	WorkerStopped
)

// Worker consumes envelopes until it becomes idle or cannot continue.
type Worker interface {
	Run(context.Context) WorkerResult
}

// WorkerConfig is the set of collaborators handed to a WorkerFactory. IdleTimeout bounds a single
// dequeue attempt: a worker that times out reports WorkerIdle and becomes a candidate for retirement.
type WorkerConfig[T any] struct {
	Queue              *Queue[T]
	Processor          Processor[T]
	PanicHandler       PanicHandler[T]
	UnsettledProcessor UnsettledProcessor[T]
	IdleTimeout        time.Duration
}

// WorkerFactory builds a Worker for a queue. Override it with WithWorkerFactory to plug in custom
// instrumentation, envelope wrapping, or an alternative consumption strategy.
type WorkerFactory[T any] func(WorkerConfig[T]) Worker

type defaultWorker[T any] struct {
	config WorkerConfig[T]
}

func defaultWorkerFactory[T any](config WorkerConfig[T]) Worker {
	return &defaultWorker[T]{config: config}
}

func (w *defaultWorker[T]) Run(ctx context.Context) WorkerResult {
	for {
		dequeueCtx, cancel := context.WithTimeout(ctx, w.config.IdleTimeout)
		envelope, err := w.config.Queue.Dequeue(dequeueCtx)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			return WorkerIdle
		}
		if err != nil {
			return WorkerStopped
		}
		w.process(ctx, envelope)
	}
}

func (w *defaultWorker[T]) process(ctx context.Context, envelope JobEnvelope[T]) {
	if renewable, ok := envelope.(AutoRenewableJobEnvelope); ok {
		interval := renewable.AutoRenewInterval()
		if interval > 0 {
			stop := renewJobEnvelope(ctx, envelope, interval)
			defer stop()
		}
	}
	cause := UnsettledCause{Kind: UnsettledReturned}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				cause = UnsettledCause{Kind: UnsettledPanicked, Panic: recovered}
				callPanicHandler(w.config.PanicHandler, ctx, w.config.Queue.ID(), envelope, recovered)
			}
		}()
		w.config.Processor(ctx, w.config.Queue.ID(), envelope)
	}()
	if !envelope.Settled() && w.config.UnsettledProcessor != nil {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					callPanicHandler(w.config.PanicHandler, ctx, w.config.Queue.ID(), envelope, recovered)
				}
			}()
			w.config.UnsettledProcessor(ctx, w.config.Queue.ID(), envelope, cause)
		}()
	}
}

func renewJobEnvelope[T any](ctx context.Context, envelope JobEnvelope[T], interval time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				if envelope.Touch(ctx) != nil {
					return
				}
			}
		}
	}()
	return func() { close(stop); <-done }
}

func callPanicHandler[T any](handler PanicHandler[T], ctx context.Context, id int, envelope JobEnvelope[T], recovered any) {
	if handler == nil {
		return
	}
	defer func() { _ = recover() }()
	handler(ctx, id, envelope, recovered)
}
