package beanstalk

import (
	"sync"
	"sync/atomic"
	"time"

	gitBeanstalk "github.com/beanstalkd/go-beanstalk"
)

// FakeManager represents a manager created for testing the queue without interacting with the beanstalk.
type FakeManager struct {
	Addr  string
	jobs  [][]byte
	mutex sync.Mutex

	// PutJobErr is the error that will be returned when calling the PutJob
	PutJobErr error
	// GetJobErr is the error that will be returned when calling the GetJob
	GetJobErr error
	// DeleteJobErr is the error that will be returned when calling the DeleteJob
	DeleteJobErr error

	TubeIsActive    atomic.Bool
	TubeSetIsActive atomic.Bool

	// Count of reconnects
	ReconnectTubeTry    atomic.Int64
	ReconnectTubeSetTry atomic.Int64

	// Count of deletions
	DeletedJobs atomic.Int64
}

func (m *FakeManager) CreateTubes(_ string) {
	m.TubeIsActive.Store(true)
	m.TubeSetIsActive.Store(true)
}

// PutJob returns an PutJobErr or adds a job to an array.
func (m *FakeManager) PutJob(body []byte, _ uint32, _, _ time.Duration) (id uint64, err error) {
	if m.PutJobErr != nil {
		return 0, m.PutJobErr
	}

	m.mutex.Lock()
	m.jobs = append(m.jobs, body)
	m.mutex.Unlock()

	return 0, nil
}

// GetJob returns an GetJobErr or return the last job from the array.
func (m *FakeManager) GetJob(_ time.Duration) (id uint64, body []byte, err error) {
	m.mutex.Lock()
	if len(m.jobs) == 0 {
		m.mutex.Unlock()
		return 0, nil, gitBeanstalk.ErrTimeout
	}

	if m.GetJobErr != nil {
		return 0, nil, m.GetJobErr
	}

	lastJob := m.jobs[len(m.jobs)-1]
	m.jobs = m.jobs[:len(m.jobs)-1]
	m.mutex.Unlock()

	return 0, lastJob, nil
}

func (m *FakeManager) DeleteJob(_ uint64) error {
	if m.DeleteJobErr != nil {
		return m.DeleteJobErr
	}

	m.DeletedJobs.Add(1)
	return nil
}

func (m *FakeManager) CloseTube() error {
	m.TubeIsActive.Store(false)
	return nil
}

func (m *FakeManager) CloseTubeSet() error {
	m.TubeSetIsActive.Store(false)
	return nil
}

func (m *FakeManager) ReconnectTube() {
	m.ReconnectTubeTry.Add(1)
}

func (m *FakeManager) ReconnectTubeSet() {
	m.ReconnectTubeSetTry.Add(1)
}
