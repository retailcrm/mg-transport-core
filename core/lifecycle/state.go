// Package lifecycle provides opt-in process lifecycle primitives for transports.
package lifecycle

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/retailcrm/mg-transport-core/v2/core"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

// Snapshot contains the number of currently active HTTP handlers and jobs.
type Snapshot struct {
	ActiveHTTPHandlers int64
	ActiveJobs         int64
}

// State tracks readiness and work admitted during the process lifetime.
type State struct {
	mu                 sync.Mutex
	ready              bool
	shuttingDown       bool
	activeJobs         int64
	jobsDone           chan struct{}
	activeHTTPHandlers atomic.Int64
	shutdownContext    context.Context
	shutdownCancel     context.CancelFunc
	stopDeadline       func() bool
}

// New creates a lifecycle state that starts unready.
func New() *State {
	ctx, cancel := context.WithCancel(context.Background())
	jobsDone := make(chan struct{})
	close(jobsDone)

	return &State{
		jobsDone:        jobsDone,
		shutdownContext: ctx,
		shutdownCancel:  cancel,
	}
}

// MarkReady sets readiness unless shutdown has already begun.
func (s *State) MarkReady() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.shuttingDown {
		s.ready = true
	}
}

// Ready reports whether the process is ready to receive traffic.
func (s *State) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ready
}

// ShuttingDown reports whether shutdown has begun.
func (s *State) ShuttingDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.shuttingDown
}

// BeginShutdown removes readiness and refuses new jobs. Outbound HTTP requests
// remain active until ctx ends or Stop is called.
func (s *State) BeginShutdown(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.shuttingDown {
		return
	}

	s.shuttingDown = true
	s.ready = false
	s.stopDeadline = context.AfterFunc(ctx, s.shutdownCancel)
}

// Stop immediately cancels outbound HTTP requests and removes readiness.
func (s *State) Stop() {
	s.mu.Lock()
	s.shuttingDown = true
	s.ready = false
	stopDeadline := s.stopDeadline
	s.stopDeadline = nil
	s.mu.Unlock()

	if stopDeadline != nil {
		stopDeadline()
	}
	s.shutdownCancel()
}

// HTTPMiddleware counts active Gin handlers without changing request handling.
func (s *State) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.activeHTTPHandlers.Add(1)
		defer s.activeHTTPHandlers.Add(-1)
		c.Next()
	}
}

// ManagementHandler exposes readiness and liveness on a separate HTTP server.
// The caller decides where to listen and when to stop that server.
func (s *State) ManagementHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readiness", func(w http.ResponseWriter, _ *http.Request) {
		if !s.Ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /liveness", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

// TrackJob wraps a job so shutdown can refuse new work and wait for admitted work.
// A job refused during shutdown returns context.Canceled.
func (s *State) TrackJob(command core.JobFunc) core.JobFunc {
	return func(log logger.Logger) error {
		s.mu.Lock()
		if s.shuttingDown {
			s.mu.Unlock()
			return context.Canceled
		}
		if s.activeJobs == 0 {
			s.jobsDone = make(chan struct{})
		}
		s.activeJobs++
		s.mu.Unlock()

		defer s.finishJob()
		return command(log)
	}
}

func (s *State) finishJob() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeJobs--
	if s.activeJobs == 0 {
		close(s.jobsDone)
	}
}

// WaitJobs waits for all jobs admitted before the call. Call BeginShutdown
// first to prevent another job from starting after the wait returns.
func (s *State) WaitJobs(ctx context.Context) error {
	s.mu.Lock()
	done := s.jobsDone
	s.mu.Unlock()

	select {
	case <-done:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Snapshot returns current active work counters.
func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	activeJobs := s.activeJobs
	s.mu.Unlock()

	return Snapshot{
		ActiveHTTPHandlers: s.activeHTTPHandlers.Load(),
		ActiveJobs:         activeJobs,
	}
}
