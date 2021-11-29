package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/retailcrm/mg-transport-core/core/logger"
)

// JobFunc is empty func which should be executed in a parallel goroutine.
type JobFunc func(logger.Logger) error

// JobErrorHandler is a function to handle jobs errors. First argument is a job name.
type JobErrorHandler func(string, error, logger.Logger)

// JobPanicHandler is a function to handle jobs panics. First argument is a job name.
type JobPanicHandler func(string, interface{}, logger.Logger)

// Job represents single job. Regular job will be executed every Interval.
type Job struct {
	Command      JobFunc
	ErrorHandler JobErrorHandler
	PanicHandler JobPanicHandler
	stopChannel  chan bool
	Interval     time.Duration
	writeLock    sync.RWMutex
	Regular      bool
	active       bool
}

// JobManager controls jobs execution flow. Jobs can be added just for later use (e.g. JobManager can be used as
// singleton), or jobs can be executed as regular jobs. Example initialization:
// 	    manager := NewJobManager().
// 			SetLogger(logger).
// 			SetLogging(false)
// 		_ = manager.RegisterJob("updateTokens", &Job{
// 			Command: func(log logger.Logger) error {
// 				// logic goes here...
// 				logger.Info("All tokens were updated successfully")
// 				return nil
// 			},
// 			ErrorHandler: DefaultJobErrorHandler(),
// 			PanicHandler: DefaultJobPanicHandler(),
// 			Interval:     time.Hour * 3,
// 			Regular:      true,
// 		})
// 		manager.Start()
type JobManager struct {
	logger        logger.Logger
	nilLogger     logger.Logger
	jobs          *sync.Map
	enableLogging bool
}

// getWrappedFunc wraps job into function.
func (j *Job) getWrappedFunc(name string, log logger.Logger) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil && j.PanicHandler != nil {
				j.PanicHandler(name, r, log)
			}
		}()

		if err := j.Command(log); err != nil && j.ErrorHandler != nil {
			j.ErrorHandler(name, err, log)
		}
	}
}

// getWrappedTimerFunc returns job timer func to run in the separate goroutine.
func (j *Job) getWrappedTimerFunc(name string, log logger.Logger) func(chan bool) {
	return func(stopChannel chan bool) {
		for range time.NewTicker(j.Interval).C {
			select {
			case <-stopChannel:
				return
			default:
				j.getWrappedFunc(name, log)()
			}
		}
	}
}

// run job.
func (j *Job) run(name string, log logger.Logger) {
	j.writeLock.RLock()

	if j.Regular && j.Interval > 0 && !j.active {
		j.writeLock.RUnlock()
		defer j.writeLock.Unlock()
		j.writeLock.Lock()

		j.stopChannel = make(chan bool)
		go j.getWrappedTimerFunc(name, log)(j.stopChannel)
		j.active = true
	} else {
		j.writeLock.RUnlock()
	}
}

// stop running job.
func (j *Job) stop() {
	j.writeLock.RLock()

	if j.active && j.stopChannel != nil {
		j.writeLock.RUnlock()
		go func() {
			defer j.writeLock.Unlock()
			j.writeLock.Lock()
			j.stopChannel <- true
			j.active = false
		}()
	} else {
		j.writeLock.RUnlock()
	}
}

// runOnce run job once.
func (j *Job) runOnce(name string, log logger.Logger) {
	go j.getWrappedFunc(name, log)()
}

// runOnceSync run job once in current goroutine.
func (j *Job) runOnceSync(name string, log logger.Logger) {
	j.getWrappedFunc(name, log)()
}

// NewJobManager is a JobManager constructor.
func NewJobManager() *JobManager {
	return &JobManager{jobs: &sync.Map{}, nilLogger: logger.NewNil()}
}

// DefaultJobErrorHandler returns default error handler for a job.
func DefaultJobErrorHandler() JobErrorHandler {
	return func(name string, err error, log logger.Logger) {
		if err != nil && name != "" {
			log.Errorf("Job `%s` errored with an error: `%s`", name, err.Error())
		}
	}
}

// DefaultJobPanicHandler returns default panic handler for a job.
func DefaultJobPanicHandler() JobPanicHandler {
	return func(name string, recoverValue interface{}, log logger.Logger) {
		if recoverValue != nil && name != "" {
			log.Errorf("Job `%s` panicked with value: `%#v`", name, recoverValue)
		}
	}
}

// SetLogger sets logger into JobManager.
func (j *JobManager) SetLogger(logger logger.Logger) *JobManager {
	if logger != nil {
		j.logger = logger
	}

	return j
}

// Logger returns logger.
func (j *JobManager) Logger() logger.Logger {
	if !j.enableLogging {
		return j.nilLogger
	}
	return j.logger
}

// SetLogging enables or disables JobManager logging.
func (j *JobManager) SetLogging(enableLogging bool) *JobManager {
	j.enableLogging = enableLogging
	return j
}

// RegisterJob registers new job.
func (j *JobManager) RegisterJob(name string, job *Job) error {
	if job == nil {
		return errors.New("job shouldn't be nil")
	}

	if _, ok := j.FetchJob(name); ok {
		return errors.New("job already exists")
	}

	j.jobs.Store(name, job)

	return nil
}

// UnregisterJob unregisters job if it's exists. Returns error if job doesn't exist.
func (j *JobManager) UnregisterJob(name string) error {
	if i, ok := j.FetchJob(name); ok {
		i.stop()
		j.jobs.Delete(name)
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// FetchJob fetches already exist job.
func (j *JobManager) FetchJob(name string) (value *Job, ok bool) {
	if i, ok := j.jobs.Load(name); ok {
		if job, ok := i.(*Job); ok {
			return job, ok
		}
	}

	return &Job{}, false
}

// UpdateJob updates job.
func (j *JobManager) UpdateJob(name string, job *Job) error {
	if _, ok := j.FetchJob(name); ok {
		_ = j.UnregisterJob(name)
		return j.RegisterJob(name, job)
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// RunJob starts provided regular job if it's exists.
// It runs asynchronously and error returns only of job wasn't executed at all.
func (j *JobManager) RunJob(name string) error {
	if job, ok := j.FetchJob(name); ok {
		job.run(name, j.Logger())
		return nil
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// StopJob stops provided regular regular job if it's exists.
func (j *JobManager) StopJob(name string) error {
	if job, ok := j.FetchJob(name); ok {
		job.stop()
		return nil
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// RunJobOnce starts provided job once if it exists. It's also async.
func (j *JobManager) RunJobOnce(name string) error {
	if job, ok := j.FetchJob(name); ok {
		job.runOnce(name, j.Logger())
		return nil
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// RunJobOnceSync starts provided job once in current goroutine if job exists. Will wait for job to end it's work.
func (j *JobManager) RunJobOnceSync(name string) error {
	if job, ok := j.FetchJob(name); ok {
		job.runOnceSync(name, j.Logger())
		return nil
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// Start all jobs in the manager.
func (j *JobManager) Start() {
	j.jobs.Range(func(key, value interface{}) bool {
		name := key.(string)
		job := value.(*Job)
		job.run(name, j.Logger())
		return true
	})
}
