package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/op/go-logging"
)

// JobFunc is empty func which should be executed in a parallel goroutine
type JobFunc func(JobLogFunc) error

// JobLogFunc is a function which logs data from job
type JobLogFunc func(string, logging.Level, ...interface{})

// JobErrorHandler is a function to handle jobs errors. First argument is a job name.
type JobErrorHandler func(string, error, JobLogFunc)

// JobPanicHandler is a function to handle jobs panics. First argument is a job name.
type JobPanicHandler func(string, interface{}, JobLogFunc)

// Job represents single job. Regular job will be executed every Interval.
type Job struct {
	Command      JobFunc
	ErrorHandler JobErrorHandler
	PanicHandler JobPanicHandler
	Interval     time.Duration
	Regular      bool
	active       bool
	stopChannel  chan bool
}

// JobManager controls jobs execution flow. Jobs can be added just for later use (e.g. JobManager can be used as
// singleton), or jobs can be executed as regular jobs. Example initialization:
// 	    manager := NewJobManager().
// 			SetLogger(logger).
// 			SetLogging(false)
// 		_ = manager.RegisterJob("updateTokens", &Job{
// 			Command: func(logFunc JobLogFunc) error {
// 				// logic goes here...
// 				logFunc("All tokens were updated successfully", logging.INFO)
// 				return nil
// 			},
// 			ErrorHandler: DefaultJobErrorHandler(),
// 			PanicHandler: DefaultJobPanicHandler(),
// 			Interval:     time.Hour * 3,
// 			Regular:      true,
// 		})
// 		manager.Start()
type JobManager struct {
	jobs          *sync.Map
	enableLogging bool
	logger        *logging.Logger
}

// getWrappedFunc wraps job into function
func (j *Job) getWrappedFunc(name string, log JobLogFunc) func() {
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

// getWrappedTimerFunc returns job timer func to run in the separate goroutine
func (j *Job) getWrappedTimerFunc(name string, log JobLogFunc) func(chan bool) {
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

// run job
func (j *Job) run(name string, log JobLogFunc) *Job {
	if j.Regular && j.Interval > 0 && !j.active {
		j.stopChannel = make(chan bool)
		go j.getWrappedTimerFunc(name, log)(j.stopChannel)
		j.active = true
	}

	return j
}

// stop running job
func (j *Job) stop() *Job {
	if j.active && j.stopChannel != nil {
		go func() {
			j.stopChannel <- true
			j.active = false
		}()
	}

	return j
}

// runOnce run job once
func (j *Job) runOnce(name string, log JobLogFunc) *Job {
	go j.getWrappedFunc(name, log)()
	return j
}

// runOnceSync run job once in current goroutine
func (j *Job) runOnceSync(name string, log JobLogFunc) *Job {
	j.getWrappedFunc(name, log)()
	return j
}

// NewJobManager is a JobManager constructor
func NewJobManager() *JobManager {
	return &JobManager{jobs: &sync.Map{}}
}

// DefaultJobErrorHandler returns default error handler for a job
func DefaultJobErrorHandler() JobErrorHandler {
	return func(name string, err error, log JobLogFunc) {
		if err != nil && name != "" {
			log("Job `%s` errored with an error: `%s`", logging.ERROR, name, err.Error())
		}
	}
}

// DefaultJobPanicHandler returns default panic handler for a job
func DefaultJobPanicHandler() JobPanicHandler {
	return func(name string, recoverValue interface{}, log JobLogFunc) {
		if recoverValue != nil && name != "" {
			log("Job `%s` panicked with value: `%#v`", logging.ERROR, name, recoverValue)
		}
	}
}

// SetLogger sets logger into JobManager
func (j *JobManager) SetLogger(logger *logging.Logger) *JobManager {
	if logger != nil {
		j.logger = logger
	}

	return j
}

// SetLogging enables or disables JobManager logging
func (j *JobManager) SetLogging(enableLogging bool) *JobManager {
	j.enableLogging = enableLogging
	return j
}

// RegisterJob registers new job
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

// FetchJob fetches already exist job
func (j *JobManager) FetchJob(name string) (value *Job, ok bool) {
	if i, ok := j.jobs.Load(name); ok {
		if job, ok := i.(*Job); ok {
			return job, ok
		}
	}

	return &Job{}, false
}

// UpdateJob updates job
func (j *JobManager) UpdateJob(name string, job *Job) error {
	if job, ok := j.FetchJob(name); ok {
		_ = j.UnregisterJob(name)
		return j.RegisterJob(name, job)
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// RunJob starts provided regular job if it's exists. It's async operation and error returns only of job wasn't executed at all.
func (j *JobManager) RunJob(name string) error {
	if job, ok := j.FetchJob(name); ok {
		job.run(name, j.log)
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
		job.runOnce(name, j.log)
		return nil
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// RunJobOnceSync starts provided job once in current goroutine if job exists. Will wait for job to end it's work.
func (j *JobManager) RunJobOnceSync(name string) error {
	if job, ok := j.FetchJob(name); ok {
		job.runOnceSync(name, j.log)
		return nil
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// Start all jobs in the manager
func (j *JobManager) Start() {
	j.jobs.Range(func(key, value interface{}) bool {
		name := key.(string)
		job := value.(*Job)
		job.run(name, j.log)
		return true
	})
}

// log logs via logger or as plaintext
func (j *JobManager) log(format string, severity logging.Level, args ...interface{}) {
	if !j.enableLogging {
		return
	}

	if j.logger != nil {
		switch severity {
		case logging.CRITICAL:
			j.logger.Criticalf(format, args...)
		case logging.ERROR:
			j.logger.Errorf(format, args...)
		case logging.WARNING:
			j.logger.Warningf(format, args...)
		case logging.NOTICE:
			j.logger.Noticef(format, args...)
		case logging.INFO:
			j.logger.Infof(format, args...)
		case logging.DEBUG:
			j.logger.Debugf(format, args...)
		}

		return
	}

	switch severity {
	case logging.CRITICAL:
		fmt.Print("[CRITICAL] ", fmt.Sprintf(format, args...))
	case logging.ERROR:
		fmt.Print("[ERROR] ", fmt.Sprintf(format, args...))
	case logging.WARNING:
		fmt.Print("[WARNING] ", fmt.Sprintf(format, args...))
	case logging.NOTICE:
		fmt.Print("[NOTICE] ", fmt.Sprintf(format, args...))
	case logging.INFO:
		fmt.Print("[INFO] ", fmt.Sprintf(format, args...))
	case logging.DEBUG:
		fmt.Print("[DEBUG] ", fmt.Sprintf(format, args...))
	}
}
