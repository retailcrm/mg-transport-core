package core

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type JobTest struct {
	suite.Suite
	job          *Job
	syncBool     bool
	executedChan chan bool
	randomNumber chan int
	executeErr   chan error
	panicValue   chan interface{}
	lastLog      string
	lastMsgLevel logging.Level
}

type JobManagerTest struct {
	suite.Suite
	manager    *JobManager
	runnerFlag bool
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
	fn("job", errors.New("test"), func(s string, level logging.Level, i ...interface{}) {
		require.Len(t, i, 2)
		assert.Equal(t, fmt.Sprintf("%s", i[1]), "test")
	})
}

func TestDefaultJobPanicHandler(t *testing.T) {
	defer func() {
		require.Nil(t, recover())
	}()

	fn := DefaultJobPanicHandler()
	require.NotNil(t, fn)
	fn("job", errors.New("test"), func(s string, level logging.Level, i ...interface{}) {
		require.Len(t, i, 2)
		assert.Equal(t, fmt.Sprintf("%s", i[1]), "test")
	})
}

func (t *JobTest) testErrorHandler() JobErrorHandler {
	return func(name string, err error, logFunc JobLogFunc) {
		t.executeErr <- err
	}
}

func (t *JobTest) testPanicHandler() JobPanicHandler {
	return func(name string, i interface{}, logFunc JobLogFunc) {
		t.panicValue <- i
	}
}

func (t *JobTest) testLogFunc() JobLogFunc {
	return func(s string, level logging.Level, i ...interface{}) {
		t.lastLog = fmt.Sprintf(s, i...)
		t.lastMsgLevel = level
	}
}

func (t *JobTest) executed(wait time.Duration, defaultVal bool) bool {
	if t.executedChan == nil {
		return defaultVal
	}

	select {
	case c := <-t.executedChan:
		return c
	case <-time.After(wait):
		return defaultVal
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
	t.clear()
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
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
	t.clear()
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
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
	t.clear()
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
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
	t.clear()
	rand.Seed(time.Now().UnixNano())
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
			t.executedChan <- true
			t.randomNumber <- rand.Int()
			return nil
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     time.Millisecond,
		Regular:      true,
	}
}

func (t *JobTest) regularSyncJob() {
	t.clear()
	rand.Seed(time.Now().UnixNano())
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
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
	fn := t.job.getWrappedFunc("job", t.testLogFunc())
	require.NotNil(t.T(), fn)
	go fn()
	assert.True(t.T(), t.executed(time.Millisecond, false))
	assert.False(t.T(), t.errored(time.Millisecond))
	assert.False(t.T(), t.panicked(time.Millisecond))
}

func (t *JobTest) Test_getWrappedFuncError() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.onceErrorJob()
	fn := t.job.getWrappedFunc("job", t.testLogFunc())
	require.NotNil(t.T(), fn)
	go fn()
	assert.True(t.T(), t.executed(time.Millisecond, false))
	assert.True(t.T(), t.errored(time.Millisecond))
	assert.False(t.T(), t.panicked(time.Millisecond))
}

func (t *JobTest) Test_getWrappedFuncPanic() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.oncePanicJob()
	fn := t.job.getWrappedFunc("job", t.testLogFunc())
	require.NotNil(t.T(), fn)
	go fn()
	assert.True(t.T(), t.executed(time.Millisecond, false))
	assert.False(t.T(), t.errored(time.Millisecond))
	assert.True(t.T(), t.panicked(time.Millisecond))
}

// func (t *JobTest) Test_getWrappedTimerFunc() {
// 	defer func() {
// 		require.Nil(t.T(), recover())
// 	}()
//
// 	t.clear()
// 	t.regularJob()
// 	t.job.run("job", t.testLogFunc())
// 	time.Sleep(time.Millisecond * 5)
// 	require.True(t.T(), t.executed(time.Millisecond, false))
// 	first := 0
//
// 	select {
// 	case c := <-t.randomNumber:
// 		first = c
// 		t.randomNumber = make(chan int)
// 	case <-time.After(time.Millisecond * 2):
// 		first = 0
// 	}
//
// 	require.NotEqual(t.T(), 0, first)
// 	second := 0
//
// 	select {
// 	case c := <-t.randomNumber:
// 		second = c
// 		t.randomNumber = make(chan int)
// 	case <-time.After(time.Millisecond * 2):
// 		second = 0
// 	}
//
// 	require.NotEqual(t.T(), 0, second)
// 	assert.NotEqual(t.T(), first, second)
// }

func (t *JobTest) Test_runOnce() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.regularJob()
	t.job.runOnce("job", t.testLogFunc())
	time.Sleep(time.Millisecond * 5)
	require.True(t.T(), t.executed(time.Millisecond, false))
	first := 0

	select {
	case c := <-t.randomNumber:
		first = c
		t.randomNumber = make(chan int)
	case <-time.After(time.Millisecond * 2):
		first = 0
	}

	require.NotEqual(t.T(), 0, first)
	second := 0

	select {
	case c := <-t.randomNumber:
		second = c
		t.randomNumber = make(chan int)
	case <-time.After(time.Millisecond * 2):
		second = 0
	}

	assert.Equal(t.T(), 0, second)
}

func (t *JobTest) Test_runOnceSync() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.regularSyncJob()
	require.False(t.T(), t.syncBool)
	t.job.runOnceSync("job", t.testLogFunc())
	assert.True(t.T(), t.syncBool)
}

func (t *JobManagerTest) SetupSuite() {
	t.manager = NewJobManager()
}

func (t *JobManagerTest) Test_SetLogger() {
	t.manager.logger = nil
	t.manager.SetLogger(NewLogger("test", logging.ERROR, DefaultLogFormatter()))
	assert.IsType(t.T(), &logging.Logger{}, t.manager.logger)

	t.manager.SetLogger(nil)
	assert.IsType(t.T(), &logging.Logger{}, t.manager.logger)
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
		Command: func(log JobLogFunc) error {
			t.runnerFlag = true
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

	require.NoError(t.T(), t.manager.RegisterJob("test_fetch", &Job{Command: func(logFunc JobLogFunc) error {
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

func (t *JobManagerTest) Test_RunOnceSync() {
	require.NotNil(t.T(), t.manager.jobs)
	t.runnerFlag = false
	err := t.manager.RunJobOnceSync("job")
	require.NoError(t.T(), err)
	assert.True(t.T(), t.runnerFlag)
}

func (t *JobManagerTest) Test_UnregisterJobDoesntExist() {
	require.NotNil(t.T(), t.manager.jobs)
	err := t.manager.UnregisterJob("doesn't exist")
	assert.EqualError(t.T(), err, "cannot find job `doesn't exist`")
}
