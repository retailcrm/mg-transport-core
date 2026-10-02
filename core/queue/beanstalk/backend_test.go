package beanstalk

import (
	"sync"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeJob struct {
	body     []byte
	attempts uint64
}
type fakeManager struct {
	mu       sync.Mutex
	jobs     map[uint64]*fakeJob
	ready    chan uint64
	nextID   uint64
	reserved int64
}

func newFakeManager() *fakeManager {
	return &fakeManager{jobs: make(map[uint64]*fakeJob), ready: make(chan uint64, 10)}
}
func (m *fakeManager) Put(body []byte, _ uint32, delay, _ time.Duration) (uint64, error) {
	m.mu.Lock()
	m.nextID++
	id := m.nextID
	m.jobs[id] = &fakeJob{body: body}
	m.mu.Unlock()
	time.AfterFunc(delay, func() { m.ready <- id })
	return id, nil
}
func (m *fakeManager) Reserve(timeout time.Duration) (uint64, []byte, error) {
	select {
	case id := <-m.ready:
		m.mu.Lock()
		job := m.jobs[id]
		job.attempts++
		m.reserved++
		m.mu.Unlock()
		return id, job.body, nil
	case <-time.After(timeout):
		return 0, nil, timeoutError{}
	}
}
func (m *fakeManager) Delete(id uint64) error {
	m.mu.Lock()
	delete(m.jobs, id)
	m.reserved--
	m.mu.Unlock()
	return nil
}
func (m *fakeManager) Release(id uint64, _ uint32, delay time.Duration) error {
	m.mu.Lock()
	m.reserved--
	m.mu.Unlock()
	time.AfterFunc(delay, func() { m.ready <- id })
	return nil
}
func (m *fakeManager) Touch(uint64) error {
	return nil
}
func (m *fakeManager) Attempts(id uint64) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id].attempts, nil
}
func (m *fakeManager) Stats() (TubeStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return TubeStats{Ready: int64(len(m.ready)), Reserved: m.reserved}, nil
}
func (m *fakeManager) Close() error {
	return nil
}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "timeout"
}

func TestDriver(t *testing.T) {
	driver := New(newFakeManager(), queue.JSONCodec[string]{}, Options{PollTimeout: time.Millisecond})
	q := queue.New(1, driver)
	require.NoError(t, q.Enqueue(t.Context(), "job", queue.WithID("transport-message-id")))
	envelope, err := q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "job", envelope.Value())
	assert.Equal(t, "transport-message-id", envelope.Metadata().ID)
	assert.Equal(t, uint64(1), envelope.Metadata().Attempt)
	require.NoError(t, envelope.Requeue(t.Context(), 0))
	envelope, err = q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(2), envelope.Metadata().Attempt)
	require.NoError(t, envelope.Touch(t.Context()))
	require.NoError(t, envelope.Ack(t.Context()))
}
