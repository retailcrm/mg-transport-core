package core

import (
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/stretchr/testify/require"
)

func TestStopJobClosesTimerPromptlyAndCanRestart(t *testing.T) {
	manager := NewJobManager()
	job := &Job{
		Command:  func(logger.Logger) error { return nil },
		Regular:  true,
		Interval: time.Hour,
	}
	require.NoError(t, manager.RegisterJob("periodic", job))
	manager.Start()
	firstStop := job.stopChannel
	require.NotNil(t, firstStop)

	require.NoError(t, manager.StopJob("periodic"))
	select {
	case <-firstStop:
	default:
		t.Fatal("periodic timer was not stopped promptly")
	}
	require.NoError(t, manager.StopJob("periodic"))
	require.False(t, job.active)

	require.NoError(t, manager.RunJob("periodic"))
	require.True(t, job.active)
	require.NotEqual(t, firstStop, job.stopChannel)
	require.NoError(t, manager.StopJob("periodic"))
}

func TestStopRegularJobsAndUnregister(t *testing.T) {
	manager := NewJobManager()
	for _, name := range []string{"first", "second"} {
		require.NoError(t, manager.RegisterJob(name, &Job{
			Command:  func(logger.Logger) error { return nil },
			Regular:  true,
			Interval: time.Hour,
		}))
	}
	require.NoError(t, manager.RegisterJob("one-shot", &Job{
		Command: func(logger.Logger) error { return nil },
	}))
	manager.Start()
	manager.StopRegularJobs()
	manager.StopRegularJobs()
	for _, name := range []string{"first", "second"} {
		job, ok := manager.FetchJob(name)
		require.True(t, ok)
		require.False(t, job.active)
		require.Nil(t, job.stopChannel)
	}

	require.NoError(t, manager.UnregisterJob("first"))
	_, exists := manager.FetchJob("first")
	require.False(t, exists)
	require.Error(t, manager.UnregisterJob("first"))
}
