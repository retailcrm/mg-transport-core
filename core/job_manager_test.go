package core

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

type JobTest struct {
	suite.Suite
	job          *Job
	executedChan chan bool
	randomNumber chan int
	executeErr   chan error
	panicValue   chan interface{}
	lastLog      string
	lastMsgLevel logging.Level
	syncBool     bool
}

type JobManagerTest struct {
	suite.Suite
	manager        *JobManager
	runnerWG       sync.WaitGroup
	syncRunnerFlag bool
}

type callbackLoggerFunc func(level logging.Level, format string, args ...interface{})

type callbackLogger struct {
	fn callbackLoggerFunc
}

func (n *callbackLogger) Fatal(args ...interface{}) {
	n.fn(logging.CRITICAL, "", args...)
}

func (n *callbackLogger) Fatalf(format string, args ...interface{}) {
	n.fn(logging.CRITICAL, format, args...)
}

func (n *callbackLogger) Panic(args ...interface{}) {
	n.fn(logging.CRITICAL, "", args...)
}
func (n *callbackLogger) Panicf(format string, args ...interface{}) {
	n.fn(logging.CRITICAL, format, args...)
}

func (n *callbackLogger) Critical(args ...interface{}) {
	n.fn(logging.CRITICAL, "", args...)
}

func (n *callbackLogger) Criticalf(format string, args ...interface{}) {
	n.fn(logging.CRITICAL, format, args...)
}

func (n *callbackLogger) Error(args ...interface{}) {
	n.fn(logging.ERROR, "", args...)
}
func (n *callbackLogger) Errorf(format string, args ...interface{}) {
	n.fn(logging.ERROR, format, args...)
}

func (n *callbackLogger) Warning(args ...interface{}) {
	n.fn(logging.WARNING, "", args...)
}
func (n *callbackLogger) Warningf(format string, args ...interface{}) {
	n.fn(logging.WARNING, format, args...)
}

func (n *callbackLogger) Notice(args ...interface{}) {
	n.fn(logging.NOTICE, "", args...)
}
func (n *callbackLogger) Noticef(format string, args ...interface{}) {
	n.fn(logging.NOTICE, format, args...)
}

func (n *callbackLogger) Info(args ...interface{}) {
	n.fn(logging.INFO, "", args...)
}
func (n *callbackLogger) Infof(format string, args ...interface{}) {
	n.fn(logging.INFO, format, args...)
}

func (n *callbackLogger) Debug(args ...interface{}) {
	n.fn(logging.DEBUG, "", args...)
}
func (n *callbackLogger) Debugf(format string, args ...interface{}) {
	n.fn(logging.DEBUG, format, args...)
}

func TestJob(t *testing.T) {
	suite.Run(t, new(JobTest))
}

func TestJobManager(t *testing.T) {
	suite.Run(t, new(JobManagerTest))
}

func TestDefaultJobErrorHandler(t *testing.T) {
	defer func() {
		require.Nil(t, recover())
	}()

	fn := DefaultJobErrorHandler()
	require.NotNil(t, fn)
	fn("job", errors.New("test"), &callbackLogger{fn: func(level logging.Level, s string, i ...interface{}) {
		require.Len(t, i, 2)
		assert.Equal(t, fmt.Sprintf("%s", i[1]), "test")
	}})
}

func TestDefaultJobPanicHandler(t *testing.T) {
	defer func() {
		require.Nil(t, recover())
	}()

	fn := DefaultJobPanicHandler()
	require.NotNil(t, fn)
	fn("job", errors.New("test"), &callbackLogger{fn: func(level logging.Level, s string, i ...interface{}) {
		require.Len(t, i, 2)
		assert.Equal(t, fmt.Sprintf("%s", i[1]), "test")
	}})
}

func (t *JobTest) testErrorHandler() JobErrorHandler {
	return func(name string, err error, log logger.Logger) {
		t.executeErr <- err
	}
}

func (t *JobTest) testPanicHandler() JobPanicHandler {
	return func(name string, i interface{}, log logger.Logger) {
		t.panicValue <- i
	}
}

func (t *JobTest) testLogger() logger.Logger {
	return &callbackLogger{fn: func(level logging.Level, format string, args ...interface{}) {
		if format == "" {
			var sb strings.Builder
			sb.Grow(3 * len(args)) // nolint:gomnd

			for i := 0; i < len(args); i++ {
				sb.WriteString("%v ")
			}

			format = strings.TrimRight(sb.String(), " ")
		}

		t.lastLog = fmt.Sprintf(format, args...)
		t.lastMsgLevel = level
	}}
}

func (t *JobTest) executed() bool {
	if t.executedChan == nil {
		return false
	}

	select {
	case c := <-t.executedChan:
		return c
	case <-time.After(time.Millisecond):
		return false
	}
}

func (t *JobTest) errored(wait time.Duration) bool {
	if t.executeErr == nil {
		return false
	}

	select {
	case c := <-t.executeErr:
		return c != nil
	case <-time.After(wait):
		return false
	}
}

func (t *JobTest) panicked(wait time.Duration) bool {
	if t.panicValue == nil {
		return false
	}

	select {
	case c := <-t.panicValue:
		return c != nil
	case <-time.After(wait):
		return false
	}
}

func (t *JobTest) clear() {
	if t.job != nil {
		t.job.stop()
		t.job = nil
	}

	t.syncBool = false
	t.randomNumber = make(chan int)
	t.executedChan = make(chan bool)
	t.executeErr = make(chan error)
	t.panicValue = make(chan interface{})
}

func (t *JobTest) onceJob() {
	t.job = &Job{
		Command: func(log logger.Logger) error {
			t.executedChan <- true
			return nil
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     0,
		Regular:      false,
	}
}

func (t *JobTest) onceErrorJob() {
	t.job = &Job{
		Command: func(log logger.Logger) error {
			t.executedChan <- true
			return errors.New("test error")
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     0,
		Regular:      false,
	}
}

func (t *JobTest) oncePanicJob() {
	t.job = &Job{
		Command: func(log logger.Logger) error {
			t.executedChan <- true
			panic("test panic")
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     0,
		Regular:      false,
	}
}

func (t *JobTest) regularJob() {
	rand.Seed(time.Now().UnixNano())
	t.job = &Job{
		Command: func(log logger.Logger) error {
			t.executedChan <- true
			t.randomNumber <- rand.Int() // nolint:gosec
			return nil
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     time.Millisecond,
		Regular:      true,
	}
}

func (t *JobTest) regularSyncJob() {
	rand.Seed(time.Now().UnixNano())
	t.job = &Job{
		Command: func(log logger.Logger) error {
			t.syncBool = true
			return nil
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     time.Millisecond,
		Regular:      true,
	}
}

func (t *JobTest) Test_getWrappedFunc() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.onceJob()
	fn := t.job.getWrappedFunc("job", t.testLogger())
	require.NotNil(t.T(), fn)
	go fn()
	assert.True(t.T(), t.executed())
	assert.False(t.T(), t.errored(time.Millisecond))
	assert.False(t.T(), t.panicked(time.Millisecond))
}

func (t *JobTest) Test_getWrappedFuncError() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.onceErrorJob()
	fn := t.job.getWrappedFunc("job", t.testLogger())
	require.NotNil(t.T(), fn)
	go fn()
	assert.True(t.T(), t.executed())
	assert.True(t.T(), t.errored(time.Millisecond))
	assert.False(t.T(), t.panicked(time.Millisecond))
}

func (t *JobTest) Test_getWrappedFuncPanic() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.oncePanicJob()
	fn := t.job.getWrappedFunc("job", t.testLogger())
	require.NotNil(t.T(), fn)
	go fn()
	assert.True(t.T(), t.executed())
	assert.False(t.T(), t.errored(time.Millisecond))
	assert.True(t.T(), t.panicked(time.Millisecond))
}

func (t *JobTest) Test_run() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.regularJob()
	t.job.run("job", t.testLogger())
	time.Sleep(time.Millisecond * 10)
	t.job.stop()
	require.True(t.T(), t.executed())
}

func (t *JobTest) Test_runOnce() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.regularJob()
	t.job.runOnce("job", t.testLogger())
	time.Sleep(time.Millisecond * 5)
	require.True(t.T(), t.executed())
	first := 0

	select {
	case c := <-t.randomNumber:
		first = c
	case <-time.After(time.Millisecond * 2):
		first = 0
	}

	second := 0

	select {
	case c := <-t.randomNumber:
		second = c
	case <-time.After(time.Millisecond * 2):
		second = 0
	}

	assert.NotEqual(t.T(), first, second)
}

func (t *JobTest) Test_runOnceSync() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.regularSyncJob()
	require.False(t.T(), t.syncBool)
	t.job.runOnceSync("job", t.testLogger())
	assert.True(t.T(), t.syncBool)
}

func (t *JobManagerTest) SetupSuite() {
	t.manager = NewJobManager()
}

func (t *JobManagerTest) WaitForJob() bool {
	c := make(chan bool)
	go func() {
		t.runnerWG.Wait()
		c <- true
	}()

	select {
	case <-c:
		return true
	case <-time.After(time.Second):
		return false
	}
}

func (t *JobManagerTest) Test_SetLogger() {
	t.manager.logger = nil
	t.manager.SetLogger(logger.NewStandard("test", logging.ERROR, logger.DefaultLogFormatter()))
	assert.IsType(t.T(), &logger.StandardLogger{}, t.manager.logger)

	t.manager.SetLogger(nil)
	assert.IsType(t.T(), &logger.StandardLogger{}, t.manager.logger)
}

func (t *JobManagerTest) Test_SetLogging() {
	t.manager.enableLogging = false
	t.manager.SetLogging(true)
	assert.True(t.T(), t.manager.enableLogging)

	t.manager.SetLogging(false)
	assert.False(t.T(), t.manager.enableLogging)
}

func (t *JobManagerTest) Test_RegisterJobNil() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RegisterJob("job", nil)
	assert.EqualError(t.T(), err, "job shouldn't be nil")
}

func (t *JobManagerTest) Test_RegisterJob() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RegisterJob("job", &Job{
		Command: func(log logger.Logger) error {
			t.runnerWG.Done()
			return nil
		},
		ErrorHandler: DefaultJobErrorHandler(),
		PanicHandler: DefaultJobPanicHandler(),
	})
	assert.NoError(t.T(), err)
	err = t.manager.RegisterJob("job_regular", &Job{
		Command: func(log logger.Logger) error {
			t.runnerWG.Done()
			return nil
		},
		ErrorHandler: DefaultJobErrorHandler(),
		PanicHandler: DefaultJobPanicHandler(),
		Regular:      true,
		Interval:     time.Millisecond,
	})
	assert.NoError(t.T(), err)
	err = t.manager.RegisterJob("job_sync", &Job{
		Command: func(log logger.Logger) error {
			t.syncRunnerFlag = true
			return nil
		},
		ErrorHandler: DefaultJobErrorHandler(),
		PanicHandler: DefaultJobPanicHandler(),
	})
	assert.NoError(t.T(), err)
}

func (t *JobManagerTest) Test_RegisterJobAlreadyExists() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RegisterJob("job", &Job{})
	assert.EqualError(t.T(), err, "job already exists")
}

func (t *JobManagerTest) Test_FetchJobDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	_, ok := t.manager.FetchJob("doesn't exist")
	assert.False(t.T(), ok)
}

func (t *JobManagerTest) Test_FetchJob() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	require.NoError(t.T(), t.manager.RegisterJob("test_fetch", &Job{Command: func(log logger.Logger) error {
		return nil
	}}))
	require.NotNil(t.T(), t.manager.jobs)
	job, ok := t.manager.FetchJob("test_fetch")
	assert.True(t.T(), ok)
	require.NotNil(t.T(), job)
	assert.NotNil(t.T(), job.Command)
	_ = t.manager.UnregisterJob("test_fetch")
}

func (t *JobManagerTest) Test_UpdateJobDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.UpdateJob("doesn't exist", &Job{})
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}

func (t *JobManagerTest) Test_UpdateJob() {
	require.NotNil(t.T(), t.manager.jobs)
	job, _ := t.manager.FetchJob("job")
	err := t.manager.UpdateJob("job", job)
	assert.NoError(t.T(), err)
}

func (t *JobManagerTest) Test_StopJobDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.StopJob("doesn't exist")
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}

func (t *JobManagerTest) Test_RunJobDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RunJob("doesn't exist")
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}

func (t *JobManagerTest) Test_RunJob_RunJobOnce() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.StopJob("job_regular")
	require.NoError(t.T(), err)
	t.runnerWG.Add(1)
	err = t.manager.RunJobOnce("job_regular")
	require.NoError(t.T(), err)
	time.Sleep(time.Millisecond)
	err = t.manager.StopJob("job_regular")
	require.NoError(t.T(), err)
	assert.True(t.T(), t.WaitForJob(), "Job was not executed in time")
}

func (t *JobManagerTest) Test_RunJobOnceDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RunJobOnce("doesn't exist")
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}

func (t *JobManagerTest) Test_RunJobOnceSyncDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RunJobOnceSync("doesn't exist")
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}

func (t *JobManagerTest) Test_RunJobOnceSync() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.RunJobOnceSync("job_sync")
	require.NoError(t.T(), err)
	assert.True(t.T(), t.syncRunnerFlag)
}

func (t *JobManagerTest) Test_UnregisterJobDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.UnregisterJob("doesn't exist")
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}

func (t *JobManagerTest) Test_Start() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	manager := NewJobManager()
	_ = manager.RegisterJob("job", &Job{
		Command: func(log logger.Logger) error {
			log.Info("alive!")
			return nil
		},
		ErrorHandler: DefaultJobErrorHandler(),
		PanicHandler: DefaultJobPanicHandler(),
	})
	manager.Start()
}
