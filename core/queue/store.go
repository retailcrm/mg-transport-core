package queue

import (
	"context"
	"sync"
	"time"
)

// Constructor constructs new queue.
type Constructor[T any] func(id int) (Queue[T], context.CancelFunc)

// WorkerConstructor constructs new worker and returns it for it to be executed.
// Supplemental task can also be started in the constructor.
type WorkerConstructor[T any] func(ctx context.Context, id int) Worker[T]

// ScaleFunc will run on each Queue in the Store every time it is called (controlled by interval in the WithScaleFunc).
// It can check queue information and call addWorker or deleteWorker to add or remove workers if necessary.
// Note that addWorker can't exceed maxNumWorkers, and deleteWorker can't stop last worker.
// Ergo, you can safely call them however much you like.
// availableScaling returns how many workers can be spawned and how many workers are currently active.
type ScaleFunc func(q Info, addWorker, deleteWorker func(), availableScaling func() (slotsLeft, slotsActive int))

// Store stores queues and manages their workers.
type Store[T any] struct {
	m                 sync.Map
	getLock           sync.Mutex
	stopsLock         sync.Mutex
	queueStopMap      map[int]*stopFuncList
	scaleLock         sync.RWMutex
	scaleFunc         ScaleFunc
	scaleInterval     time.Duration
	queueConstructor  Constructor[T]
	numWorkers        int
	maxNumWorkers     int
	workerConstructor WorkerConstructor[T]
}

// stopFuncList stores main queue context cancellation function and workers cancellation functions.
// This should be refactored to simpler logic later (e.g. single context & cancelFunc).
type stopFuncList struct {
	queue   context.CancelFunc
	workers []context.CancelFunc
}

// NewStore is a store constructor.
func NewStore[T any](constructor Constructor[T]) *Store[T] {
	return &Store[T]{queueConstructor: constructor, numWorkers: 1, queueStopMap: map[int]*stopFuncList{}}
}

// WithWorkerConstructor will set worker constructor for workers.
func (s *Store[T]) WithWorkerConstructor(f WorkerConstructor[T]) *Store[T] {
	s.workerConstructor = f
	return s
}

// WithNumWorkers specifies number of workers to start with.
func (s *Store[T]) WithNumWorkers(n int) *Store[T] {
	s.numWorkers = n
	if s.maxNumWorkers < s.numWorkers {
		s.maxNumWorkers = s.numWorkers
	}
	return s
}

// WithMaxNumWorkers specifies maximum number of workers for each queue.
func (s *Store[T]) WithMaxNumWorkers(n int) *Store[T] {
	s.maxNumWorkers = n
	return s
}

// WithScaleFunc will start scaling func each checkInterval.
func (s *Store[T]) WithScaleFunc(fn ScaleFunc, checkInterval time.Duration) *Store[T] {
	s.scaleLock.Lock()
	s.scaleFunc = fn
	s.scaleInterval = checkInterval
	s.scaleLock.Unlock()

	s.m.Range(func(key, value any) bool {
		id, idOk := key.(int)
		queue, queueOk := value.(Info)
		if idOk && queueOk {
			go s.performAutoScale(queue.Context(), id, queue, fn, checkInterval)
		}
		return true
	})

	return s
}

// Get queue from the store.
func (s *Store[T]) Get(id int) Queue[T] {
	if val, exists := s.m.Load(id); exists {
		return val.(Queue[T])
	}

	s.getLock.Lock()
	defer s.getLock.Unlock()

	if val, exists := s.m.Load(id); exists {
		return val.(Queue[T])
	}

	q, stop := s.queueConstructor(id)
	s.m.Store(id, q)
	s.spawnWorkers(id, stop, q)
	s.spawnAutoScale(id, q)

	return q
}

// Remove queue from the store.
func (s *Store[T]) Remove(id int) {
	s.getLock.Lock()
	defer s.getLock.Unlock()

	s.invokeStoppers(id)
	s.m.Delete(id)
}

// performAutoScale will call ScaleFunc for the queue every checkInterval until the queue context is canceled.
func (s *Store[T]) performAutoScale(
	ctx context.Context, id int, queue Info, scaleFunc ScaleFunc, checkInterval time.Duration) {
	defer func() {
		if r := recover(); r != nil && ctx.Err() == nil {
			go s.performAutoScale(ctx, id, queue, scaleFunc, checkInterval)
			return
		}
	}()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			scaleFunc(queue, func() {
				s.addWorker(id)
			}, func() {
				s.stopWorker(id)
			}, func() (int, int) {
				return s.scalingInfo(id)
			})
		}
	}
}

func (s *Store[T]) spawnAutoScale(id int, q Queue[T]) {
	s.scaleLock.RLock()
	scaleFunc := s.scaleFunc
	checkInterval := s.scaleInterval
	s.scaleLock.RUnlock()

	if scaleFunc == nil || checkInterval <= 0 {
		return
	}

	go s.performAutoScale(q.Context(), id, q, scaleFunc, checkInterval)
}

// addStoppers adds stop functions to the storage.
func (s *Store[T]) addStoppers(id int, queueStop context.CancelFunc, stopFuncs []context.CancelFunc) {
	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	if stops, ok := s.queueStopMap[id]; ok {
		if len(stopFuncs) > 0 {
			stopFuncs = append(stops.workers, stopFuncs...)
			s.queueStopMap[id].workers = stopFuncs
		}
		if queueStop != nil {
			s.queueStopMap[id].queue = queueStop
		}
		return
	}
	s.queueStopMap[id] = &stopFuncList{queue: queueStop, workers: stopFuncs}
}

// addWorker spawns additional worker for the queue with specified id.
// It can't spawn more than maxNumWorkers workers.
func (s *Store[T]) addWorker(id int) {
	if s.numWorkers == s.maxNumWorkers {
		return
	}
	if s.workerConstructor == nil {
		return
	}
	q, exists := s.m.Load(id)
	if !exists {
		return
	}
	queue, ok := q.(Queue[T])
	if !ok {
		return
	}

	s.stopsLock.Lock()
	defer s.stopsLock.Unlock()

	stops, ok := s.queueStopMap[id]
	if !ok || stops == nil {
		wCtx, cancel := context.WithCancel(queue.Context())
		worker := s.workerConstructor(wCtx, id)
		s.queueStopMap[id] = &stopFuncList{workers: []context.CancelFunc{cancel}}
		go worker(queue)
		return
	}

	if len(stops.workers) >= s.maxNumWorkers {
		return
	}

	wCtx, cancel := context.WithCancel(queue.Context())
	worker := s.workerConstructor(wCtx, id)

	stops.workers = append(stops.workers, cancel)
	go worker(queue)
}

// stopWorker destroys worker for the queue with specified id.
// It can't stop the last worker.
func (s *Store[T]) stopWorker(id int) {
	if s.numWorkers == s.maxNumWorkers {
		return
	}
	_, exists := s.m.Load(id)
	if !exists {
		return
	}

	s.stopsLock.Lock()
	stops, ok := s.queueStopMap[id]

	if !ok || stops == nil {
		s.stopsLock.Unlock()
		return
	}
	if len(stops.workers) <= 1 {
		s.stopsLock.Unlock()
		return
	}

	cancel := stops.workers[len(stops.workers)-1]
	s.queueStopMap[id].workers = stops.workers[:len(stops.workers)-1]
	s.stopsLock.Unlock()

	cancel()
}

// scalingInfo returns how many additional workers can be spawned and how many of them are active.
func (s *Store[T]) scalingInfo(id int) (slotsLeft, slotsActive int) {
	if s.numWorkers == s.maxNumWorkers {
		return 0, s.numWorkers
	}

	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	stops, ok := s.queueStopMap[id]
	if !ok || stops == nil {
		return s.maxNumWorkers, 0
	}
	active := len(stops.workers)
	return s.maxNumWorkers - active, active
}

// invokeStoppers stops the queue and all workers for the queue with specified id.
func (s *Store[T]) invokeStoppers(id int) {
	s.stopsLock.Lock()
	stops, ok := s.queueStopMap[id]
	if !ok || stops == nil {
		s.stopsLock.Unlock()
		return
	}
	queueStop := stops.queue
	workerStops := append([]context.CancelFunc(nil), stops.workers...)
	delete(s.queueStopMap, id)
	s.stopsLock.Unlock()

	if queueStop != nil {
		queueStop()
	}

	// Если вдруг у очереди контекст без отмены был, то вручную отменяем контекст у воркеров
	for _, fn := range workerStops {
		fn()
	}
}

// spawnWorkers spawns initial workers for newly added queue.
func (s *Store[T]) spawnWorkers(id int, stopQueue context.CancelFunc, q Queue[T]) {
	if s.workerConstructor == nil {
		s.addStoppers(id, stopQueue, nil)
		return
	}

	stoppers := make([]context.CancelFunc, s.numWorkers)
	for i := 0; i < s.numWorkers; i++ {
		wCtx, cancel := context.WithCancel(q.Context())

		worker := s.workerConstructor(wCtx, id)
		stoppers[i] = cancel
		go worker(q)
	}
	s.addStoppers(id, stopQueue, stoppers)
}
