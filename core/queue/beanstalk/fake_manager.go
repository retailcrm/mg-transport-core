package beanstalk

import (
	"sync"
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

	TubeIsActive    bool
	TubeSetIsActive bool

	// Count of reconnects
	ReconnectTubeTry    int
	ReconnectTubeSetTry int

	// Count of deletions
	DeletedJobs int
}

func (m *FakeManager) CreateTubes(_ string) {
	m.TubeIsActive = true
	m.TubeSetIsActive = true
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

	m.DeletedJobs++
	return nil
}

func (m *FakeManager) CloseTube() error {
	m.TubeIsActive = false
	return nil
}

func (m *FakeManager) CloseTubeSet() error {
	m.TubeSetIsActive = false
	return nil
}

func (m *FakeManager) ReconnectTube() {
	m.ReconnectTubeTry++
}

func (m *FakeManager) ReconnectTubeSet() {
	m.ReconnectTubeSetTry++
}
