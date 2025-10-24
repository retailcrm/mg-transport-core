package beanstalk

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/beanstalkd/go-beanstalk"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/tevino/abool/v2"
)

type QueueProcessorFunc = func(id uint64, body []byte, done func())

// Queue represents a beanstalk based queue.
type Queue struct {
	manager  ManagerInterface
	jobTtr   time.Duration
	sendLock sync.Mutex
	recvLock sync.RWMutex
	stop     *abool.AtomicBool
	log      logger.Logger
}

// New creates a new Queue with the specified ManagerInterface, ttr for beanstalk jobs, name for tubes, and logger.
func New(manager ManagerInterface, jobTtr time.Duration, name string, log logger.Logger) *Queue {
	q := &Queue{
		manager: manager,
		jobTtr:  jobTtr,
		stop:    abool.New(),
		log:     log,
	}
	q.manager.CreateTubes(name)

	return q
}

// Put adds a job to the queue.
func (q *Queue) Put(v interface{}) (uint64, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	id, err := q.manager.PutJob(encoded, 1, 0, q.jobTtr)
	if err != nil {
		q.log.Error("cannot enqueue a job", zap.Error(err), zap.Any("message", v))
		if q.isNetConnErr(err) {
			if q.stop.IsSet() {
				return 0, fmt.Errorf("queue was stopped")
			}
			q.sendLock.Lock()
			q.manager.ReconnectTube()
			q.sendLock.Unlock()

			return q.Put(v)
		}
		return 0, err
	}
	return id, nil
}

// Process adds a job to the queue.
func (q *Queue) Process(fn QueueProcessorFunc) {
	for {
		id, body, err := q.manager.GetJob(time.Second)
		if err != nil {
			if errors.Is(err, beanstalk.ErrTimeout) {
				continue
			}
			if q.stop.IsSet() {
				return
			}
			q.manager.ReconnectTubeSet()
			continue
		}

		go fn(id, body, func() {
			q.finishJob(id)
		})
	}
}

// Shutdown stops the queue process.
func (q *Queue) Shutdown() {
	q.stop.Set()
	_ = q.manager.CloseTube()
	_ = q.manager.CloseTubeSet()
}

// finishJob removes a job from the queue.
func (q *Queue) finishJob(uid uint64) {
	for {
		q.recvLock.RLock()
		err := q.manager.DeleteJob(uid)

		if err != nil {
			q.log.Error(fmt.Sprintf("cannot delete job id=%d", uid), zap.Error(err))
		} else {
			q.log.Debug(fmt.Sprintf("deleted job id=%d", uid))
		}

		if err != nil && q.isNetConnErr(err) {
			q.log.Error("error while deleting job", zap.Error(err))
			if q.stop.IsSet() {
				return
			}
			q.recvLock.RUnlock()
			q.recvLock.Lock()
			q.manager.ReconnectTubeSet()
			q.recvLock.Unlock()
			continue
		}

		q.recvLock.RUnlock()
		break
	}
}

// isNetConnErr checks if the error is of net.Error.
func (q *Queue) isNetConnErr(err error) bool {
	for {
		if err == nil {
			return false
		}
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}
		err = errors.Unwrap(err)
	}
}
