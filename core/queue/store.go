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
type WorkerConstructor[T any] func(id int) (Worker[T], context.CancelFunc)

// ScaleFunc will run on each Queue in the Store every time it is called (controlled by interval in the WithScaleFunc).
// It can check queue information and call addWorker or deleteWorker to add or remove workers if necessary.
// Note that addWorker can't exceed maxNumWorkers, and deleteWorker can't stop last worker.
// Ergo, you can safely call them however much you like.
// availableScaling returns how many workers can be spawned and how many workers are currently active.
type ScaleFunc func(q Info, addWorker, deleteWorker func(), availableScaling func() (slotsLeft, slotsActive int))

// Store stores queues and manages their workers.
type Store[T any] struct {
	m                 sync.Map
	stopsLock         sync.Mutex
	stops             map[int]*stopFuncList
	constructor       Constructor[T]
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
	return &Store[T]{constructor: constructor, numWorkers: 1, stops: map[int]*stopFuncList{}}
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
	go s.performAutoScale(fn, checkInterval)
	return s
}

// Get queue from the store.
func (s *Store[T]) Get(id int) Queue[T] {
	val, exists := s.m.Load(id)
	if !exists {
		q, stop := s.constructor(id)
		s.m.Store(id, q)
		s.spawnWorkers(id, stop, q)
		return q
	}
	return val.(Queue[T])
}

// Remove queue from the store.
func (s *Store[T]) Remove(id int) {
	s.invokeStoppers(id)
	s.m.Delete(id)
}

// performAutoScale will call ScaleFunc on each queue every checkInterval.
func (s *Store[T]) performAutoScale(scaleFunc ScaleFunc, checkInterval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			go s.performAutoScale(scaleFunc, checkInterval)
			return
		}
	}()
	for {
		time.Sleep(checkInterval)
		s.m.Range(func(key, value any) bool {
			id, ok := key.(int)
			if !ok {
				return true
			}
			queue, ok := value.(Info)
			if !ok {
				return true
			}
			scaleFunc(queue, func() {
				s.addWorker(id)
			}, func() {
				s.stopWorker(id)
			}, func() (int, int) {
				return s.scalingInfo(id)
			})
			return true
		})
	}
}

// addStoppers adds stop functions to the storage.
func (s *Store[T]) addStoppers(id int, queueStop context.CancelFunc, stopFuncs []context.CancelFunc) {
	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	if stops, ok := s.stops[id]; ok {
		if len(stopFuncs) > 0 {
			stopFuncs = append(stops.workers, stopFuncs...)
			s.stops[id].workers = stopFuncs
		}
		if queueStop != nil {
			s.stops[id].queue = queueStop
		}
		return
	}
	s.stops[id] = &stopFuncList{queue: queueStop, workers: stopFuncs}
}

// addWorker spawns additional worker for the queue with specified id.
// It can't spawn more than maxNumWorkers workers.
func (s *Store[T]) addWorker(id int) {
	if s.numWorkers == s.maxNumWorkers {
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

	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	stops, ok := s.stops[id]
	if !ok || stops == nil {
		worker, stop := s.workerConstructor(id)
		s.stops[id] = &stopFuncList{workers: []context.CancelFunc{stop}}
		go worker(queue)
		return
	}
	if len(stops.workers) >= s.maxNumWorkers {
		return
	}
	worker, stop := s.workerConstructor(id)
	stops.workers = append(stops.workers, stop)
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

	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	stops, ok := s.stops[id]
	if !ok || stops == nil {
		return
	}
	if len(stops.workers) <= 1 {
		return
	}
	stops.workers[len(stops.workers)-1]()
	s.stops[id].workers = stops.workers[:len(stops.workers)-1]
}

// scalingInfo returns how many additional workers can be spawned and how many of them are active.
func (s *Store[T]) scalingInfo(id int) (slotsLeft, slotsActive int) {
	if s.numWorkers == s.maxNumWorkers {
		return 0, s.numWorkers
	}

	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	stops, ok := s.stops[id]
	if !ok || stops == nil {
		return s.maxNumWorkers, 0
	}
	active := len(stops.workers)
	return s.maxNumWorkers - active, active
}

// invokeStoppers stops the queue and all workers for the queue with specified id.
func (s *Store[T]) invokeStoppers(id int) {
	defer s.stopsLock.Unlock()
	s.stopsLock.Lock()
	if stops, ok := s.stops[id]; ok {
		if stops == nil {
			return
		}
		if stops.queue != nil {
			stops.queue()
		}
		for _, fn := range stops.workers {
			fn()
		}
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
		worker, stop := s.workerConstructor(id)
		stoppers[i] = stop
		go worker(q)
	}
	s.addStoppers(id, stopQueue, stoppers)
}
