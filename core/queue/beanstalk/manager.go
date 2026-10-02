package beanstalk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	beanstalk "github.com/beanstalkd/go-beanstalk"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"go.uber.org/zap"
)

// TubeStats is a snapshot of the tube job counters used for queue statistics.
type TubeStats struct {
	Ready    int64
	Delayed  int64
	Reserved int64
}

// ManagerInterface is the subset of beanstalkd operations required by a Driver. It is implemented by
// Manager and can be satisfied by test doubles.
type ManagerInterface interface {
	Put([]byte, uint32, time.Duration, time.Duration) (uint64, error)
	Reserve(time.Duration) (uint64, []byte, error)
	Delete(uint64) error
	Release(uint64, uint32, time.Duration) error
	Touch(uint64) error
	Attempts(uint64) (uint64, error)
	Stats() (TubeStats, error)
	Close() error
}

// Manager maintains a producer connection and a pool of consumer connections. A reserved job keeps
// its consumer connection until settlement, as required by the beanstalkd protocol. Reserve and
// producer operations reconnect after network errors. All methods are safe for concurrent use.
type Manager struct {
	address        string
	tubeName       string
	log            logger.Logger
	reconnectDelay time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	closed         atomic.Bool
	sendGate       chan struct{}
	receiveMu      sync.Mutex
	tube           *beanstalk.Tube
	idle           []*consumerConn
	consumers      map[*consumerConn]struct{}
	inFlight       map[uint64]*consumerConn
}

type consumerConn struct {
	mu      sync.Mutex
	tubeSet *beanstalk.TubeSet
	conn    atomic.Pointer[beanstalk.Conn]
}

// NewManager dials the beanstalkd server at address and binds one producer and one consumer
// connection to the tube. It retries dialing until the context is canceled; reconnectDelay throttles
// the retry loop (default one second). A nil log falls back to a no-op logger.
func NewManager(ctx context.Context, address, tube string, log logger.Logger, reconnectDelay time.Duration) (*Manager, error) {
	if reconnectDelay <= 0 {
		reconnectDelay = time.Second
	}
	if log == nil {
		log = logger.NewNil()
	}
	runtimeCtx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		address: address, tubeName: tube, log: log, reconnectDelay: reconnectDelay,
		ctx: runtimeCtx, cancel: cancel, sendGate: make(chan struct{}, 1),
	}
	manager.sendGate <- struct{}{}
	if err := manager.connect(ctx); err != nil {
		cancel()
		return nil, err
	}
	return manager, nil
}

func (m *Manager) dial(ctx context.Context) (*beanstalk.Conn, error) {
	for {
		conn, err := beanstalk.Dial("tcp", m.address)
		if err == nil {
			return conn, nil
		}
		m.log.Warn("cannot connect to beanstalkd", zap.Error(err))
		timer := time.NewTimer(m.reconnectDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (m *Manager) connect(ctx context.Context) error {
	producer, err := m.dial(ctx)
	if err != nil {
		return err
	}
	consumer, err := m.dial(ctx)
	if err != nil {
		_ = producer.Close()
		return err
	}
	m.tube = beanstalk.NewTube(producer, m.tubeName)
	session := &consumerConn{tubeSet: beanstalk.NewTubeSet(consumer, m.tubeName)}
	session.conn.Store(consumer)
	m.idle = []*consumerConn{session}
	m.consumers = map[*consumerConn]struct{}{session: {}}
	m.inFlight = make(map[uint64]*consumerConn)
	return nil
}

func (m *Manager) Put(body []byte, priority uint32, delay, ttr time.Duration) (uint64, error) {
	return m.PutContext(context.Background(), body, priority, delay, ttr)
}

// PutContext puts a job and stops retrying when ctx or the manager is canceled.
func (m *Manager) PutContext(ctx context.Context, body []byte, priority uint32, delay, ttr time.Duration) (uint64, error) {
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	defer cancel()
	if err := m.acquireSend(ctx); err != nil {
		return 0, err
	}
	defer m.releaseSend()
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		connection := m.tube.Conn
		stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
		id, err := m.tube.Put(body, priority, delay, ttr)
		stop()
		if !isNetworkError(err) || m.closed.Load() {
			return id, err
		}
		if err := m.reconnectProducerLocked(ctx); err != nil {
			return 0, err
		}
	}
}
func (m *Manager) Reserve(timeout time.Duration) (uint64, []byte, error) {
	session, err := m.checkoutConsumer()
	if err != nil {
		return 0, nil, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	for {
		id, body, err := session.tubeSet.Reserve(timeout)
		if !isNetworkError(err) || m.closed.Load() {
			m.receiveMu.Lock()
			if m.closed.Load() {
				m.receiveMu.Unlock()
				return 0, nil, context.Canceled
			}
			if err == nil {
				m.inFlight[id] = session
			} else {
				m.idle = append(m.idle, session)
			}
			m.receiveMu.Unlock()
			return id, body, err
		}
		if err := m.reconnectConsumer(session); err != nil {
			return 0, nil, err
		}
	}
}
func (m *Manager) Delete(id uint64) error {
	return m.settle(id, func(conn *beanstalk.Conn) error { return conn.Delete(id) })
}
func (m *Manager) Release(id uint64, priority uint32, delay time.Duration) error {
	return m.settle(id, func(conn *beanstalk.Conn) error { return conn.Release(id, priority, delay) })
}
func (m *Manager) Touch(id uint64) error {
	session, err := m.reservation(id)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !m.ownsReservation(id, session) {
		return beanstalk.ErrNotFound
	}
	err = session.tubeSet.Conn.Touch(id)
	if errors.Is(err, beanstalk.ErrNotFound) || isNetworkError(err) {
		m.finishConsumer(id, session, !isNetworkError(err))
	}
	return err
}
func (m *Manager) Attempts(id uint64) (uint64, error) {
	session, err := m.reservation(id)
	if err != nil {
		return 0, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !m.ownsReservation(id, session) {
		return 0, beanstalk.ErrNotFound
	}
	values, err := session.tubeSet.Conn.StatsJob(id)
	if err != nil {
		if errors.Is(err, beanstalk.ErrNotFound) || isNetworkError(err) {
			m.finishConsumer(id, session, !isNetworkError(err))
		}
		return 0, err
	}
	return m.statistic[uint64](values, "reserves")
}
func (m *Manager) Stats() (TubeStats, error) {
	if err := m.acquireSend(m.ctx); err != nil {
		return TubeStats{}, err
	}
	defer m.releaseSend()
	connection := m.tube.Conn
	stop := context.AfterFunc(m.ctx, func() { _ = connection.Close() })
	defer stop()
	values, err := m.tube.Stats()
	if err != nil {
		return TubeStats{}, err
	}
	ready, err := m.statistic[int64](values, "current-jobs-ready")
	if err != nil {
		return TubeStats{}, err
	}
	delayed, err := m.statistic[int64](values, "current-jobs-delayed")
	if err != nil {
		return TubeStats{}, err
	}
	reserved, err := m.statistic[int64](values, "current-jobs-reserved")
	if err != nil {
		return TubeStats{}, err
	}
	return TubeStats{Ready: ready, Delayed: delayed, Reserved: reserved}, nil
}
func (m *Manager) Close() error {
	m.closed.Store(true)
	m.cancel()
	_ = m.acquireSend(context.Background())
	m.receiveMu.Lock()
	defer m.releaseSend()
	defer m.receiveMu.Unlock()
	var errs []error
	errs = append(errs, m.tube.Conn.Close())
	for session := range m.consumers {
		errs = append(errs, session.conn.Load().Close())
	}
	return errors.Join(errs...)
}

func (m *Manager) acquireSend(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.sendGate:
		return nil
	}
}

func (m *Manager) releaseSend() {
	m.sendGate <- struct{}{}
}

func (m *Manager) statistic[Integer ~int64 | ~uint64](values map[string]string, key string) (Integer, error) {
	value, err := strconv.ParseUint(values[key], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse beanstalk statistic %s: %w", key, err)
	}
	return Integer(value), nil
}

func (m *Manager) checkoutConsumer() (*consumerConn, error) {
	m.receiveMu.Lock()
	if m.closed.Load() {
		m.receiveMu.Unlock()
		return nil, context.Canceled
	}
	if len(m.idle) > 0 {
		last := len(m.idle) - 1
		session := m.idle[last]
		m.idle = m.idle[:last]
		m.receiveMu.Unlock()
		return session, nil
	}
	m.receiveMu.Unlock()
	conn, err := m.dial(m.ctx)
	if err != nil {
		return nil, err
	}
	session := &consumerConn{tubeSet: beanstalk.NewTubeSet(conn, m.tubeName)}
	session.conn.Store(conn)
	m.receiveMu.Lock()
	defer m.receiveMu.Unlock()
	if m.closed.Load() {
		_ = conn.Close()
		return nil, context.Canceled
	}
	m.consumers[session] = struct{}{}
	return session, nil
}

func (m *Manager) reservation(id uint64) (*consumerConn, error) {
	m.receiveMu.Lock()
	defer m.receiveMu.Unlock()
	session := m.inFlight[id]
	if session == nil {
		return nil, beanstalk.ErrNotFound
	}
	return session, nil
}

func (m *Manager) ownsReservation(id uint64, session *consumerConn) bool {
	m.receiveMu.Lock()
	defer m.receiveMu.Unlock()
	return m.inFlight[id] == session
}

func (m *Manager) finishConsumer(id uint64, session *consumerConn, reusable bool) {
	m.receiveMu.Lock()
	delete(m.inFlight, id)
	if reusable && !m.closed.Load() {
		m.idle = append(m.idle, session)
	} else if !reusable {
		delete(m.consumers, session)
		_ = session.conn.Load().Close()
	}
	m.receiveMu.Unlock()
}

func (m *Manager) settle(id uint64, operation func(*beanstalk.Conn) error) error {
	session, err := m.reservation(id)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !m.ownsReservation(id, session) {
		return beanstalk.ErrNotFound
	}
	err = operation(session.tubeSet.Conn)
	if err == nil || errors.Is(err, beanstalk.ErrNotFound) || isNetworkError(err) {
		m.finishConsumer(id, session, !isNetworkError(err))
	}
	return err
}

func (m *Manager) reconnectProducerLocked(ctx context.Context) error {
	_ = m.tube.Conn.Close()
	connection, err := m.dial(ctx)
	if err != nil {
		return err
	}
	m.tube = beanstalk.NewTube(connection, m.tubeName)
	return nil
}

func (m *Manager) reconnectConsumer(session *consumerConn) error {
	_ = session.tubeSet.Conn.Close()
	connection, err := m.dial(m.ctx)
	if err != nil {
		return err
	}
	m.receiveMu.Lock()
	defer m.receiveMu.Unlock()
	if m.closed.Load() {
		_ = connection.Close()
		return context.Canceled
	}
	session.tubeSet = beanstalk.NewTubeSet(connection, m.tubeName)
	session.conn.Store(connection)
	return nil
}

func isNetworkError(err error) bool {
	_, ok := errors.AsType[net.Error](err)
	return ok || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed)
}
