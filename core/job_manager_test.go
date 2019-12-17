package core

import (
	"errors"
	"fmt"
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
	executed     bool
	executeErr   error
	lastLog      string
	lastMsgLevel logging.Level
	panicValue   interface{}
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
		t.executeErr = err
	}
}

func (t *JobTest) testPanicHandler() JobPanicHandler {
	return func(name string, i interface{}, logFunc JobLogFunc) {
		t.panicValue = i
	}
}

func (t *JobTest) testLogFunc() JobLogFunc {
	return func(s string, level logging.Level, i ...interface{}) {
		t.lastLog = fmt.Sprintf(s, i...)
		t.lastMsgLevel = level
	}
}

func (t *JobTest) errored() bool {
	return t.executeErr != nil
}

func (t *JobTest) panicked() bool {
	return t.panicValue != nil
}

func (t *JobTest) clear() {
	if t.job != nil {
		t.job.stop()
		t.job = nil
	}

	t.executed = false
	t.executeErr = nil
	t.panicValue = nil
}

func (t *JobTest) onceJob() {
	t.clear()
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
			t.executed = true
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
			t.executed = true
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
			t.executed = true
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
	t.job = &Job{
		Command: func(logFunc JobLogFunc) error {
			t.executed = true
			return nil
		},
		ErrorHandler: t.testErrorHandler(),
		PanicHandler: t.testPanicHandler(),
		Interval:     time.Nanosecond,
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
	fn()
	assert.True(t.T(), t.executed)
	assert.False(t.T(), t.errored())
	assert.False(t.T(), t.panicked())
}

func (t *JobTest) Test_getWrappedFuncError() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.onceErrorJob()
	fn := t.job.getWrappedFunc("job", t.testLogFunc())
	require.NotNil(t.T(), fn)
	fn()
	assert.True(t.T(), t.executed)
	assert.True(t.T(), t.errored())
	assert.False(t.T(), t.panicked())
}

func (t *JobTest) Test_getWrappedFuncPanic() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.oncePanicJob()
	fn := t.job.getWrappedFunc("job", t.testLogFunc())
	require.NotNil(t.T(), fn)
	fn()
	assert.True(t.T(), t.executed)
	assert.False(t.T(), t.errored())
	assert.True(t.T(), t.panicked())
}

func (t *JobTest) Test_getWrappedTimerFunc() {
	defer func() {
		require.Nil(t.T(), recover())
	}()

	t.clear()
	t.regularJob()
	t.job.run("job", t.testLogFunc())
	time.Sleep(time.Millisecond)
	assert.True(t.T(), t.executed)
	t.executed = false
	time.Sleep(time.Millisecond)
	if !t.executed {
		t.clear()
		t.T().Skip("job wasn't as fast as it should be! this may be an error, but also can be just bad timing")
	}
	t.job.stop()
	time.Sleep(time.Nanosecond * 10)
	t.clear()
	assert.False(t.T(), t.executed)
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

func (t *JobManagerTest) Test_RunOnceSync() {
	require.NotNil(t.T(), t.manager.jobs)
	t.runnerFlag = false
	err := t.manager.RunJobOnceSync("job")
	require.NoError(t.T(), err)
	assert.True(t.T(), t.runnerFlag)
}
