package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/op/go-logging"
)

// JobFunc is empty func which should be executed in a parallel goroutine
type JobFunc func() error

// JobErrorHandler is a function to handle jobs errors. First argument is a job name.
type JobErrorHandler func(string, error, *logging.Logger)

// JobPanicHandler is a function to handle jobs panics. First argument is a job name.
type JobPanicHandler func(string, interface{}, *logging.Logger)

// Job represents single job. Regular job will be executed every Interval.
type Job struct {
	Command      JobFunc
	ErrorHandler JobErrorHandler
	PanicHandler JobPanicHandler
	Regular      bool
	Interval     time.Duration
	lastExecuted time.Time
}

// JobManager controls jobs execution flow. Jobs can be added just for later use (e.g. JobManager can be used as
// singleton), or jobs can be executed as regular jobs. Example initialization:
// TODO example initialization
type JobManager struct {
	jobs             *sync.Map
	enableLogging    bool
	logger           *logging.Logger
	executorInterval time.Duration
	executorChannel  chan bool
}

// NewJobManager is a JobManager constructor
func NewJobManager() *JobManager {
	return &JobManager{jobs: &sync.Map{}}
}

// DefaultExecutorInterval is a default recommended interval for main job executor
func DefaultExecutorInterval() time.Duration {
	return time.Minute
}

// DefaultJobErrorHandler is a default error handler for a job
func DefaultJobErrorHandler(name string, err error, logger *logging.Logger) {
	if err != nil && name != "" {
		message := fmt.Sprintf("Job `%s` errored with an error: `%s`", name, err.Error())

		if logger != nil {
			logger.Error(message)
		} else {
			fmt.Print("[ERROR]", message)
		}
	}
}

// DefaultJobPanicHandler is a default panic handler for a job
func DefaultJobPanicHandler(name string, recoverValue interface{}, logger *logging.Logger) {
	if recoverValue != nil && name != "" {
		message := fmt.Sprintf("Job `%s` panicked with value: `%#v`", name, recoverValue)

		if logger != nil {
			logger.Error(message)
		} else {
			fmt.Print("[ERROR]", message)
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
func (j *JobManager) RegisterJob(name string, job Job) error {
	if i, ok := j.jobs.Load(name); ok {
		if _, ok := j.asJob(i); ok {
			return errors.New("job already exists")
		}
	}

	j.jobs.Store(name, job)
	return nil
}

// FetchJob fetches already exist job
func (j *JobManager) FetchJob(name string) (value Job, ok bool) {
	if i, ok := j.jobs.Load(name); ok {
		if job, ok := j.asJob(i); ok {
			return job, ok
		}
	}

	return Job{}, false
}

// UpdateJob updates job
func (j *JobManager) UpdateJob(name string, job Job) error {
	if job, ok := j.FetchJob(name); ok {
		j.jobs.Delete(name)
		return j.RegisterJob(name, job)
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// ExecuteJob executes provided job if it's exists. It's async operation and error returns only of job wasn't executed at all.
func (j *JobManager) ExecuteJob(name string, resetInterval bool) error {
	if job, ok := j.FetchJob(name); ok {
		return j.runJob(name, job, resetInterval)
	}

	return fmt.Errorf("cannot find job `%s`", name)
}

// StartExecutor runs executor
func (j *JobManager) StartExecutor(executorInterval time.Duration) error {
	if executorInterval <= 0 {
		return errors.New("executorInterval must be higher that 0")
	}

	if j.executorChannel != nil {
		return errors.New("executor is already active")
	}

	j.executorInterval = executorInterval
	j.executorChannel = make(chan bool)

	go func(stop chan bool) {
		for _ = range time.NewTicker(j.executorInterval).C {
			select {
			case <-stop:
				return
			case <-time.After(time.Second):
				j.jobs.Range(func(key, value interface{}) bool {
					if job, ok := j.asJob(value); ok {
						if name, ok := key.(string); ok {
							if job.Regular &&
								job.lastExecuted.Before(time.Now()) &&
								time.Since(job.lastExecuted) >= job.Interval {
								if err := j.runJob(name, job, true); err != nil {
									j.logError("error while executing job `%s`: %s", name, err.Error())
								}
							}
						}
					}

					return true
				})
			}
		}
	}(j.executorChannel)

	return nil
}

// logError logs error
func (j *JobManager) logError(format string, args ...interface{}) {
	if j.logger != nil {
		j.logger.Errorf(format, args...)
	}

	fmt.Printf(format, args...)
}

// asJob casts interface to a Job
func (j *JobManager) asJob(v interface{}) (Job, bool) {
	if job, ok := v.(Job); ok {
		return job, ok
	}

	return Job{}, false
}

// runJob executes provided job from object. It's async operation and error returns only of job wasn't executed at all.
func (j *JobManager) runJob(name string, job Job, resetInterval bool) error {
	go func() {
		defer func() {
			if r := recover(); r != nil && job.PanicHandler != nil {
				job.PanicHandler(name, r, j.logger)
			}
		}()

		if err := job.Command(); err != nil && job.ErrorHandler != nil {
			job.ErrorHandler(name, err, j.logger)
		}
	}()

	if resetInterval {
		job.lastExecuted = time.Now()
		return j.UpdateJob(name, job)
	}

	return fmt.Errorf("cannot find job `%s`", name)
}
