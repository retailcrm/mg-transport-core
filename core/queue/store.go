package queue

import (
	"context"
	"fmt"
	"sync"
)

// Constructor constructs a queue and returns its stop function.
type Constructor[T any] func(id int) (Queue[T], context.CancelFunc)

// Store keeps queue executors by queue ID.
type Store[T any] struct {
	mu            sync.RWMutex
	executors     map[int]*QueueExecutor[T]
	constructor   Constructor[T]
	processor     ProcessFunc[T]
	panicHandler  PanicHandler[T]
	workerFactory WorkerFactory[T]
	policy        WorkerPolicy
	stopped       bool
}

// NewStore constructs a store. It returns an error for an invalid worker policy.
func NewStore[T any](
	constructor Constructor[T],
	processor ProcessFunc[T],
	policy WorkerPolicy,
	options ...StoreOption[T],
) (*Store[T], error) {
	if constructor == nil {
		return nil, fmt.Errorf("queue constructor is required")
	}
	if processor == nil {
		return nil, fmt.Errorf("processor is required")
	}

	if err := policy.validate(); err != nil {
		return nil, err
	}

	s := &Store[T]{
		executors:     make(map[int]*QueueExecutor[T]),
		constructor:   constructor,
		processor:     processor,
		workerFactory: defaultWorkerFactory[T],
		policy:        policy,
	}
	for _, option := range options {
		option(s)
	}
	if s.workerFactory == nil {
		return nil, fmt.Errorf("worker factory is required")
	}

	return s, nil
}

// StoreOption configures a Store.
type StoreOption[T any] func(*Store[T])

// WithPanicHandler configures handling for panics raised by the processor.
func WithPanicHandler[T any](handler PanicHandler[T]) StoreOption[T] {
	return func(s *Store[T]) {
		s.panicHandler = handler
	}
}

// WithWorkerFactory configures worker construction.
func WithWorkerFactory[T any](factory WorkerFactory[T]) StoreOption[T] {
	return func(s *Store[T]) {
		s.workerFactory = factory
	}
}

// Get returns the queue executor for id, creating it if necessary.
func (s *Store[T]) Get(id int) (*QueueExecutor[T], error) {
	return s.getOrCreate(id)
}

// Info returns queue executor information for id.
func (s *Store[T]) Info(id int) (QueueExecutorInfo, bool) {
	s.mu.RLock()
	executor, ok := s.executors[id]
	s.mu.RUnlock()
	if !ok {
		return QueueExecutorInfo{}, false
	}

	return executor.Info(), true
}

// Remove stops and removes the queue executor for id.
func (s *Store[T]) Remove(id int) {
	s.mu.Lock()
	executor, ok := s.executors[id]
	if ok {
		delete(s.executors, id)
	}
	s.mu.Unlock()

	if ok {
		executor.stop()
	}
}

// Stop stops all queue executors and prevents creation of new ones.
func (s *Store[T]) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}

	s.stopped = true
	executors := make([]*QueueExecutor[T], 0, len(s.executors))
	for _, executor := range s.executors {
		executors = append(executors, executor)
	}
	clear(s.executors)
	s.mu.Unlock()

	for _, executor := range executors {
		executor.stop()
	}
}

func (s *Store[T]) getOrCreate(id int) (*QueueExecutor[T], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil, context.Canceled
	}
	if executor, ok := s.executors[id]; ok {
		return executor, nil
	}

	executor := newQueueExecutor(
		id,
		s.constructor,
		s.processor,
		s.policy,
		s.panicHandler,
		s.workerFactory,
	)
	s.executors[id] = executor

	return executor, nil
}
