package lifecycle

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/stretchr/testify/require"
)

func TestManagementHandlerAndReadiness(t *testing.T) {
	state := New()
	handler := state.ManagementHandler()
	checkStatus := func(path string, want int) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, want, response.Code)
	}

	checkStatus("/readiness", http.StatusServiceUnavailable)
	checkStatus("/liveness", http.StatusOK)
	checkStatus("/webhook", http.StatusNotFound)
	state.MarkReady()
	checkStatus("/readiness", http.StatusOK)

	state.BeginShutdown(t.Context())
	state.MarkReady()
	require.True(t, state.ShuttingDown())
	require.False(t, state.Ready())
	checkStatus("/readiness", http.StatusServiceUnavailable)
	checkStatus("/liveness", http.StatusOK)
	state.Stop()
}

func TestTrackJobWaitsAndRejectsAfterShutdown(t *testing.T) {
	state := New()
	started := make(chan struct{})
	release := make(chan struct{})
	job := state.TrackJob(func(logger.Logger) error {
		close(started)
		<-release
		return nil
	})
	jobDone := make(chan error, 1)
	go func() { jobDone <- job(nil) }()
	<-started

	state.BeginShutdown(t.Context())
	require.Equal(t, int64(1), state.Snapshot().ActiveJobs)
	refused := state.TrackJob(func(logger.Logger) error {
		t.Error("job started after shutdown")
		return nil
	})
	require.ErrorIs(t, refused(nil), context.Canceled)

	waitDone := make(chan error, 1)
	go func() { waitDone <- state.WaitJobs(t.Context()) }()
	select {
	case <-waitDone:
		t.Fatal("wait returned before the job finished")
	default:
	}
	close(release)
	require.NoError(t, <-jobDone)
	require.NoError(t, <-waitDone)
	require.Zero(t, state.Snapshot().ActiveJobs)
	state.Stop()
}

func TestWaitJobsDeadlineAndPanicCleanup(t *testing.T) {
	state := New()
	started := make(chan struct{})
	release := make(chan struct{})
	job := state.TrackJob(func(logger.Logger) error {
		close(started)
		<-release
		panic("job failed")
	})
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		_ = job(nil)
	}()
	<-started

	state.BeginShutdown(t.Context())
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, state.WaitJobs(ctx), context.DeadlineExceeded)
	close(release)
	require.Equal(t, "job failed", <-panicked)
	require.NoError(t, state.WaitJobs(t.Context()))
	require.Zero(t, state.Snapshot().ActiveJobs)
	state.Stop()
}

func TestConcurrentReadinessAndShutdown(t *testing.T) {
	state := New()
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() { state.MarkReady() })
	}
	state.BeginShutdown(t.Context())
	workers.Wait()
	require.False(t, state.Ready())
	state.Stop()
}

func TestConcurrentJobAdmissionAndShutdown(t *testing.T) {
	state := New()
	const jobCount = 32
	start := make(chan struct{})
	release := make(chan struct{})
	var started atomic.Int64
	job := state.TrackJob(func(logger.Logger) error {
		started.Add(1)
		<-release
		return nil
	})
	results := make(chan error, jobCount)
	var workers sync.WaitGroup
	for range jobCount {
		workers.Go(func() {
			<-start
			results <- job(nil)
		})
	}
	close(start)
	state.BeginShutdown(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if started.Load() > 0 {
		require.ErrorIs(t, state.WaitJobs(ctx), context.DeadlineExceeded)
	}
	close(release)
	workers.Wait()
	close(results)
	var completed int64
	for err := range results {
		if err == nil {
			completed++
		} else {
			require.ErrorIs(t, err, context.Canceled)
		}
	}
	require.Equal(t, started.Load(), completed)
	require.NoError(t, state.WaitJobs(t.Context()))
	require.Zero(t, state.Snapshot().ActiveJobs)
	state.Stop()
}

func TestHTTPMiddlewareCountsHandlers(t *testing.T) {
	state := New()
	started := make(chan struct{})
	release := make(chan struct{})
	router := gin.New()
	router.Use(state.HTTPMiddleware())
	router.GET("/", func(c *gin.Context) {
		close(started)
		<-release
		c.Status(http.StatusNoContent)
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	<-started
	require.Equal(t, int64(1), state.Snapshot().ActiveHTTPHandlers)
	close(release)
	<-done
	require.Zero(t, state.Snapshot().ActiveHTTPHandlers)
}
