package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const drainPollInterval = 10 * time.Millisecond

// DriverConstructor builds a Driver for the given queue ID. It is called lazily when a Store creates
// an executor, which lets transports bind the driver configuration (codec, subject, tube name) to the
// account the queue serves.
type DriverConstructor[T any] func(context.Context, int) (Driver[T], error)

// StoreOption configures a Store at construction time.
type StoreOption[T any] func(*Store[T])

// WithPanicHandler registers a handler invoked with the recovered value whenever a processor or an
// unsettled processor panics.
func WithPanicHandler[T any](handler PanicHandler[T]) StoreOption[T] {
	return func(store *Store[T]) { store.panicHandler = handler }
}

// WithUnsettledProcessor registers a processor invoked for envelopes that finished processing without
// an explicit settlement, including envelopes abandoned because of a processor panic.
func WithUnsettledProcessor[T any](processor UnsettledProcessor[T]) StoreOption[T] {
	return func(store *Store[T]) { store.unsettled = processor }
}

// WithWorkerFactory replaces the default worker implementation for all executors of the store.
func WithWorkerFactory[T any](factory WorkerFactory[T]) StoreOption[T] {
	return func(store *Store[T]) { store.workerFactory = factory }
}

type storeEntry[T any] struct {
	ready    chan struct{}
	executor *Executor[T]
	err      error
	removed  bool
}

// Store manages one Executor per numeric queue ID (usually a transport account ID). Executors are
// created lazily on first use through a DriverConstructor and removed by Remove or Reconcile. The
// store applies the same processor, worker policy, and options to every executor.
//
// All Store methods are safe for concurrent use.
type Store[T any] struct {
	mu                sync.RWMutex
	executors         map[int]*storeEntry[T]
	driverConstructor DriverConstructor[T]
	processor         Processor[T]
	policy            WorkerPolicy
	panicHandler      PanicHandler[T]
	unsettled         UnsettledProcessor[T]
	workerFactory     WorkerFactory[T]
	closing           []*storeEntry[T]
	stopped           bool
	intakeClosed      bool
	localDraining     bool
}

// NewStore creates a store from a driver constructor, a processor shared by all executors, and a
// worker policy. Optional StoreOption values can register panic and unsettled-envelope handling or a
// custom worker factory. The constructor returns an error when required arguments are missing or the
// policy is invalid.
func NewStore[T any](constructor DriverConstructor[T], processor Processor[T], policy WorkerPolicy,
	options ...StoreOption[T],
) (*Store[T], error) {
	if constructor == nil {
		return nil, errors.New("driver constructor is required")
	}
	if processor == nil {
		return nil, errors.New("processor is required")
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	store := &Store[T]{
		executors: make(map[int]*storeEntry[T]), driverConstructor: constructor,
		processor: processor, policy: policy, workerFactory: defaultWorkerFactory[T],
	}
	for _, option := range options {
		option(store)
	}
	if store.workerFactory == nil {
		return nil, errors.New("worker factory is required")
	}
	return store, nil
}

// Get returns the executor for the given queue ID, creating it on first use. Concurrent callers for the
// same ID block until the driver is constructed; construction failures are returned to every waiter
// and do not leave a broken entry behind. It returns context.Canceled after Stop.
func (s *Store[T]) Get(ctx context.Context, id int) (*Executor[T], error) {
	for {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return nil, context.Canceled
		}
		if s.localDraining {
			s.mu.Unlock()
			return nil, ErrIntakeClosed
		}
		if entry := s.executors[id]; entry != nil {
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-entry.ready:
				s.mu.RLock()
				defer s.mu.RUnlock()
				if entry.removed {
					return nil, context.Canceled
				}
				return entry.executor, entry.err
			}
		}
		entry := &storeEntry[T]{ready: make(chan struct{})}
		s.executors[id] = entry
		s.mu.Unlock()

		driver, err := s.driverConstructor(ctx, id)
		if err != nil {
			s.finishConstruction(id, entry, nil, err)
			return nil, err
		}
		if driver == nil {
			err = errors.New("driver constructor returned nil driver")
			s.finishConstruction(id, entry, nil, err)
			return nil, err
		}
		executor := newExecutor(id, driver, s.processor, s.policy, s.panicHandler, s.unsettled, s.workerFactory)
		if err := s.finishConstruction(id, entry, executor, nil); err != nil {
			_ = executor.shutdown(context.WithoutCancel(ctx))
			return nil, err
		}
		return executor, nil
	}
}

func (s *Store[T]) finishConstruction(id int, entry *storeEntry[T], executor *Executor[T], constructionErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if constructionErr != nil {
		entry.err = constructionErr
		if s.executors[id] == entry {
			delete(s.executors, id)
		}
		close(entry.ready)
		return constructionErr
	}
	if s.stopped || entry.removed || s.executors[id] != entry {
		entry.err = context.Canceled
		entry.executor = executor
		close(entry.ready)
		return context.Canceled
	}
	if s.intakeClosed {
		executor.CloseIntake()
	}
	entry.executor = executor
	close(entry.ready)
	return nil
}

// Enqueue resolves (and lazily creates) the executor for the queue ID and enqueues the value there.
func (s *Store[T]) Enqueue(ctx context.Context, id int, value T, options ...EnqueueOption) error {
	executor, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return executor.Enqueue(ctx, value, options...)
}

// Info returns the executor snapshot for the queue ID. The second result reports whether an executor
// exists; a false value carries no error.
func (s *Store[T]) Info(ctx context.Context, id int) (ExecutorInfo, bool, error) {
	s.mu.RLock()
	entry := s.executors[id]
	s.mu.RUnlock()
	if entry == nil {
		return ExecutorInfo{}, false, nil
	}
	select {
	case <-ctx.Done():
		return ExecutorInfo{}, false, ctx.Err()
	case <-entry.ready:
	}
	s.mu.RLock()
	executor, entryErr, removed := entry.executor, entry.err, entry.removed
	s.mu.RUnlock()
	if entryErr != nil || executor == nil || removed {
		return ExecutorInfo{}, false, entryErr
	}
	info, err := executor.Info(ctx)
	return info, true, err
}

// Has reports whether an executor exists for the queue ID, including executors still constructing.
func (s *Store[T]) Has(id int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.executors[id] != nil
}

// Reconcile aligns the executor set with the desired list of queue IDs: missing executors are created
// and executors whose IDs are absent from the list are removed and closed. Call it periodically (for
// example from a job) to keep queues in sync with the active transport accounts. Duplicate IDs in the
// list are ignored.
func (s *Store[T]) Reconcile(ctx context.Context, ids []int) error {
	desired := make(map[int]struct{}, len(ids))
	var errs []error
	for _, id := range ids {
		if _, exists := desired[id]; exists {
			continue
		}
		desired[id] = struct{}{}
		if _, err := s.Get(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("create queue %d: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	s.mu.RLock()
	stale := make([]int, 0, len(s.executors))
	for id := range s.executors {
		if _, keep := desired[id]; !keep {
			stale = append(stale, id)
		}
	}
	s.mu.RUnlock()
	for _, id := range stale {
		if err := s.Remove(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("remove queue %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// Remove closes and forgets the executor for the queue ID. It waits for a concurrently running
// construction to finish so the executor is not leaked. Removing an unknown ID is a no-op.
func (s *Store[T]) Remove(ctx context.Context, id int) error {
	s.mu.Lock()
	entry := s.executors[id]
	if entry != nil {
		entry.removed = true
		delete(s.executors, id)
	}
	s.mu.Unlock()
	if entry == nil {
		return nil
	}
	select {
	case <-entry.ready:
	default:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-entry.ready:
		}
	}
	if entry.executor == nil {
		return entry.err
	}
	return entry.executor.Close(ctx)
}

// CloseIntake closes the intake of every current executor and of executors created afterwards. It is
// the first phase of a graceful shutdown; follow it with Drain or DrainLocal, then Stop.
func (s *Store[T]) CloseIntake() {
	s.mu.Lock()
	s.intakeClosed = true
	executors := s.readyExecutorsLocked()
	s.mu.Unlock()
	for _, executor := range executors {
		executor.CloseIntake()
	}
}

// Stats aggregates the workload counters of every executor. Errors of individual drivers are joined;
// counters of failed executors are skipped.
func (s *Store[T]) Stats(ctx context.Context) (Stats, error) {
	s.mu.RLock()
	entries := make([]*storeEntry[T], 0, len(s.executors))
	for _, entry := range s.executors {
		entries = append(entries, entry)
	}
	s.mu.RUnlock()
	var total Stats
	var errs []error
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return total, errors.Join(append(errs, ctx.Err())...)
		case <-entry.ready:
		}
		s.mu.RLock()
		executor, entryErr, removed := entry.executor, entry.err, entry.removed
		s.mu.RUnlock()
		if executor == nil || entryErr != nil || removed {
			continue
		}
		info, err := executor.Info(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		total.Ready += info.Stats.Ready
		total.Deferred += info.Stats.Deferred
		total.InFlight += info.Stats.InFlight
	}
	return total, errors.Join(errs...)
}

// Drain blocks until no executor's driver reports queued or in-flight items, or until the context
// expires. For shared durable consumers these are global counts, including other replicas' work.
// Use DrainLocal for rolling shutdown of one process.
func (s *Store[T]) Drain(ctx context.Context) error {
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	for {
		stats, err := s.Stats(ctx)
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

// DrainLocal closes local intake, stops this store's workers from fetching new envelopes, and
// waits for their current Run calls to finish. It does not wait for shared driver queues to empty.
// Call Stop afterward to close the drivers. Concurrent Get calls cannot create new executors once
// local draining starts. Custom workers must return from Run when their queue's Dequeue is canceled.
func (s *Store[T]) DrainLocal(ctx context.Context) error {
	s.mu.Lock()
	s.intakeClosed = true
	s.localDraining = true
	entries := make([]*storeEntry[T], 0, len(s.executors)+len(s.closing))
	for _, entry := range s.executors {
		entries = append(entries, entry)
	}
	entries = append(entries, s.closing...)
	ready := s.readyExecutorsLocked()
	s.mu.Unlock()

	for _, executor := range ready {
		executor.quiesceLocal()
	}
	executors := make([]*Executor[T], 0, len(entries))
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-entry.ready:
		}
		if entry.executor != nil {
			entry.executor.quiesceLocal()
			executors = append(executors, entry.executor)
		}
	}
	for _, executor := range executors {
		if err := executor.workers.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop closes every executor (worker groups first, then drivers) and renders the store unusable:
// subsequent Get calls return context.Canceled. Stop is idempotent until it succeeds; it fails fast
// when the context expires during shutdown, leaving the store in the stopped state.
func (s *Store[T]) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		s.closing = make([]*storeEntry[T], 0, len(s.executors))
		for _, entry := range s.executors {
			entry.removed = true
			s.closing = append(s.closing, entry)
		}
		clear(s.executors)
	}
	entries := append([]*storeEntry[T](nil), s.closing...)
	s.mu.Unlock()

	var errs []error
	for _, entry := range entries {
		select {
		case <-entry.ready:
		default:
			select {
			case <-ctx.Done():
				return errors.Join(append(errs, ctx.Err())...)
			case <-entry.ready:
			}
		}
		if entry.executor != nil {
			errs = append(errs, entry.executor.Close(ctx))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	s.mu.Lock()
	clear(s.closing)
	s.closing = nil
	s.mu.Unlock()
	return nil
}

func (s *Store[T]) readyExecutorsLocked() []*Executor[T] {
	executors := make([]*Executor[T], 0, len(s.executors))
	for _, entry := range s.executors {
		select {
		case <-entry.ready:
			if entry.executor != nil && entry.err == nil {
				executors = append(executors, entry.executor)
			}
		default:
		}
	}
	return executors
}
