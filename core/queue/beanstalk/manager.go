package beanstalk

import (
	"fmt"
	"time"

	gitBeanstalk "github.com/beanstalkd/go-beanstalk"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"go.uber.org/zap"
)

// Manager is a manager of interaction with Beanstalk.
type Manager struct {
	addr           string
	log            logger.Logger
	reconnectDelay time.Duration
	tube           *gitBeanstalk.Tube
	tubeSet        *gitBeanstalk.TubeSet
}

// NewManager creates a new Manager with the specified address for connecting to beanstalk,
// the logger and the reconnection delay.
func NewManager(addr string, log logger.Logger, reconnectDelay time.Duration) Manager {
	manager := Manager{
		addr:           addr,
		log:            log,
		reconnectDelay: reconnectDelay,
	}

	return manager
}

// getConnection returns a new connection to the beanstalk.
func (m *Manager) getConnection() *gitBeanstalk.Conn {
	for {
		conn, err := gitBeanstalk.Dial("tcp", m.addr)
		if err != nil {
			m.log.Info(fmt.Sprintf("cannot connect to beanstalkd, retrying in %s",
				m.reconnectDelay.String()),
				zap.Error(err),
			)
			time.Sleep(m.reconnectDelay)
			continue
		}
		return conn
	}
}

// CreateTubes returns a new Tube and TubeSet with representing the given name.
func (m *Manager) CreateTubes(name string) {
	m.tube = gitBeanstalk.NewTube(m.getConnection(), name)
	m.tubeSet = gitBeanstalk.NewTubeSet(m.getConnection(), name)
}

// PutJob put job into the beanstalk.
func (m *Manager) PutJob(body []byte, pri uint32, delay, ttr time.Duration) (id uint64, err error) {
	return m.tube.Put(body, pri, delay, ttr)
}

// GetJob attempts to get a job from the beanstalk within the specified time.
func (m *Manager) GetJob(timeout time.Duration) (id uint64, body []byte, err error) {
	return m.tubeSet.Reserve(timeout)
}

// DeleteJob removes a job from a beanstalk.
func (m *Manager) DeleteJob(id uint64) error {
	return m.tubeSet.Conn.Delete(id)
}

// CloseTube closes the underlying network connection for tube.
func (m *Manager) CloseTube() error {
	return m.tube.Conn.Close()
}

// CloseTubeSet closes the underlying network connection for tubeSet.
func (m *Manager) CloseTubeSet() error {
	return m.tubeSet.Conn.Close()
}

// ReconnectTube recreates the underlying network connection for tube.
func (m *Manager) ReconnectTube() {
	_ = m.CloseTube()
	time.Sleep(time.Millisecond)
	m.tube.Conn = m.getConnection()
}

// ReconnectTubeSet recreates the underlying network connection for tubeSet.
func (m *Manager) ReconnectTubeSet() {
	_ = m.CloseTubeSet()
	time.Sleep(time.Millisecond)
	m.tubeSet.Conn = m.getConnection()
}
