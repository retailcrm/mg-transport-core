package healthcheck

import "sync"

// SyncMapStorage is a default Storage implementation. It uses sync.Map under the hood because
// deletions should be rare for the storage. If your business logic calls Remove often, it would be better
// to use your own implementation with map[int]Counter and sync.RWMutex.
type SyncMapStorage struct {
	constructor CounterConstructor
	m           sync.Map
}

// NewSyncMapStorage is a SyncMapStorage constructor.
func NewSyncMapStorage(constructor CounterConstructor) Storage {
	return &SyncMapStorage{constructor: constructor}
}

func (s *SyncMapStorage) Get(id int, name string) Counter {
	val, found := s.m.Load(id)
	if found {
		counter := val.(Counter)
		counter.SetName(name)
		return val.(Counter)
	}
	c := s.constructor(name)
	s.m.Store(id, c)
	return c
}

func (s *SyncMapStorage) Remove(id int) {
	s.m.Delete(id)
}

func (s *SyncMapStorage) Process(proc Processor) {
	s.m.Range(func(key, value any) bool {
		return proc.Process(key.(int), value.(Counter))
	})
}
