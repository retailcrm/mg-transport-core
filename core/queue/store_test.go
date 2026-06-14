package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_ValidatesDependenciesAndPolicy(t *testing.T) {
	validPolicy := WorkerPolicy{
		MinWorkers:    1,
		MaxWorkers:    1,
		JobsPerWorker: 1,
		IdleTimeout:   time.Second,
	}

	_, err := NewStore[int](nil, func(context.Context, int) {}, validPolicy)
	require.EqualError(t, err, "queue constructor is required")

	_, err = NewStore[int](NewMemory[int], nil, validPolicy)
	require.EqualError(t, err, "processor is required")

	_, err = NewStore(
		NewMemory[int],
		func(context.Context, int) {},
		validPolicy,
		WithWorkerFactory[int](nil),
	)
	require.EqualError(t, err, "worker factory is required")

	tests := []struct {
		name   string
		policy WorkerPolicy
		err    string
	}{
		{
			name: "minimum",
			policy: WorkerPolicy{
				MaxWorkers:    1,
				JobsPerWorker: 1,
				IdleTimeout:   time.Second,
			},
			err: "min workers must be at least 1",
		},
		{
			name: "maximum",
			policy: WorkerPolicy{
				MinWorkers:    2,
				MaxWorkers:    1,
				JobsPerWorker: 1,
				IdleTimeout:   time.Second,
			},
			err: "max workers must be greater than or equal to min workers",
		},
		{
			name: "jobs per worker",
			policy: WorkerPolicy{
				MinWorkers:  1,
				MaxWorkers:  1,
				IdleTimeout: time.Second,
			},
			err: "jobs per worker must be at least 1",
		},
		{
			name: "idle timeout",
			policy: WorkerPolicy{
				MinWorkers:    1,
				MaxWorkers:    1,
				JobsPerWorker: 1,
			},
			err: "idle timeout must be positive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewStore(NewMemory[int], func(context.Context, int) {}, test.policy)
			require.EqualError(t, err, test.err)
		})
	}
}

func TestStore_ConcurrentEnqueueCreatesQueueExecutorOnce(t *testing.T) {
	var constructed atomic.Int32
	constructor := func(id int) (Queue[int], context.CancelFunc) {
		constructed.Add(1)
		time.Sleep(10 * time.Millisecond)
		return NewMemory[int](id)
	}

	store := newTestStore(t, constructor, func(context.Context, int) {}, WorkerPolicy{
		MinWorkers:    1,
		MaxWorkers:    1,
		JobsPerWorker: 1,
		IdleTimeout:   time.Second,
	})
	t.Cleanup(store.Stop)

	const goroutines = 25
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(item int) {
			defer wg.Done()
			executor, err := store.Get(1)
			require.NoError(t, err)
			require.NoError(t, executor.Enqueue(item))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int32(1), constructed.Load())
	info, ok := store.Info(1)
	require.True(t, ok)
	assert.Equal(t, 1, info.Queue.ID())
	assert.Equal(t, 1, info.ActiveWorkers)
}

func TestStore_StartsMinimumWorkers(t *testing.T) {
	store := newTestStore(t, NewMemory[int], func(context.Context, int) {}, WorkerPolicy{
		MinWorkers:    2,
		MaxWorkers:    4,
		JobsPerWorker: 10,
		IdleTimeout:   20 * time.Millisecond,
	})
	t.Cleanup(store.Stop)

	executor, err := store.Get(1)
	require.NoError(t, err)
	require.NoError(t, executor.Enqueue(1))
	require.Eventually(t, func() bool {
		info, ok := store.Info(1)
		return ok && info.ActiveWorkers == 2
	}, time.Second, 5*time.Millisecond)

	time.Sleep(80 * time.Millisecond)
	info, ok := store.Info(1)
	require.True(t, ok)
	assert.Equal(t, 2, info.ActiveWorkers)
}

func TestStore_ScalesUpAndRetiresIdleWorkers(t *testing.T) {
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var processed atomic.Int32

	store := newTestStore(t, NewMemory[int], func(context.Context, int) {
		started <- struct{}{}
		<-release
		processed.Add(1)
	}, WorkerPolicy{
		MinWorkers:    1,
		MaxWorkers:    4,
		JobsPerWorker: 1,
		IdleTimeout:   30 * time.Millisecond,
	})

	executor, err := store.Get(1)
	require.NoError(t, err)
	for i := 0; i < 8; i++ {
		require.NoError(t, executor.Enqueue(i))
	}

	require.Eventually(t, func() bool {
		info, ok := store.Info(1)
		return ok && info.ActiveWorkers == 4
	}, time.Second, 5*time.Millisecond)

	for i := 0; i < 4; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start processing")
		}
	}

	close(release)
	require.Eventually(t, func() bool {
		return processed.Load() == 8
	}, time.Second, 5*time.Millisecond)
	require.Eventually(t, func() bool {
		info, ok := store.Info(1)
		return ok && info.ActiveWorkers == 1
	}, time.Second, 5*time.Millisecond)

	store.Stop()
}

func TestStore_RecoversProcessorPanic(t *testing.T) {
	processed := make(chan int, 1)
	recovered := make(chan int, 1)

	store, err := NewStore(
		NewMemory[int],
		func(_ context.Context, item int) {
			if item == 1 {
				panic("processor panic")
			}
			processed <- item
		},
		WorkerPolicy{
			MinWorkers:    1,
			MaxWorkers:    1,
			JobsPerWorker: 1,
			IdleTimeout:   time.Second,
		},
		WithPanicHandler(func(_ context.Context, item int, recoveredValue any) {
			assert.Equal(t, "processor panic", recoveredValue)
			recovered <- item
		}),
	)
	require.NoError(t, err)
	t.Cleanup(store.Stop)

	executor, err := store.Get(1)
	require.NoError(t, err)
	require.NoError(t, executor.Enqueue(1))
	require.NoError(t, executor.Enqueue(2))

	select {
	case item := <-recovered:
		assert.Equal(t, 1, item)
	case <-time.After(time.Second):
		t.Fatal("processor panic was not recovered")
	}
	select {
	case item := <-processed:
		assert.Equal(t, 2, item)
	case <-time.After(time.Second):
		t.Fatal("worker did not continue after panic")
	}
}

func TestStore_UsesWorkerFactory(t *testing.T) {
	var factoryCalls atomic.Int32
	var defaultProcessorCalls atomic.Int32
	processed := make(chan int, 1)

	store, err := NewStore(
		NewMemory[int],
		func(context.Context, int) {
			defaultProcessorCalls.Add(1)
		},
		WorkerPolicy{
			MinWorkers:    1,
			MaxWorkers:    1,
			JobsPerWorker: 1,
			IdleTimeout:   time.Second,
		},
		WithWorkerFactory(func(config WorkerConfig[int]) Worker {
			factoryCalls.Add(1)
			return &testWorker{
				queue:       config.Queue,
				idleTimeout: config.IdleTimeout,
				processed:   processed,
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(store.Stop)

	executor, err := store.Get(42)
	require.NoError(t, err)
	require.NoError(t, executor.Enqueue(100))

	select {
	case item := <-processed:
		assert.Equal(t, 100, item)
	case <-time.After(time.Second):
		t.Fatal("custom worker did not process item")
	}

	assert.Equal(t, int32(1), factoryCalls.Load())
	assert.Zero(t, defaultProcessorCalls.Load())
}

func TestStore_RecoversWorkerPanic(t *testing.T) {
	store, err := NewStore(
		NewMemory[int],
		func(context.Context, int) {},
		WorkerPolicy{
			MinWorkers:    1,
			MaxWorkers:    1,
			JobsPerWorker: 1,
			IdleTimeout:   time.Second,
		},
		WithWorkerFactory(func(WorkerConfig[int]) Worker {
			return panicWorker{}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(store.Stop)

	_, err = store.Get(1)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, ok := store.Info(1)
		return ok && info.ActiveWorkers == 0
	}, time.Second, 5*time.Millisecond)
}

func TestStore_RemoveStopsAndRecreatesQueueExecutor(t *testing.T) {
	store := newTestStore(t, NewMemory[int], func(context.Context, int) {}, WorkerPolicy{
		MinWorkers:    1,
		MaxWorkers:    1,
		JobsPerWorker: 1,
		IdleTimeout:   time.Second,
	})
	t.Cleanup(store.Stop)

	firstExecutor, err := store.Get(1)
	require.NoError(t, err)
	require.NoError(t, firstExecutor.Enqueue(1))
	first, ok := store.Info(1)
	require.True(t, ok)

	store.Remove(1)
	_, ok = store.Info(1)
	assert.False(t, ok)
	firstQueue := first.Queue.(Queue[int])
	require.ErrorIs(t, firstQueue.Enqueue(2), context.Canceled)

	secondExecutor, err := store.Get(1)
	require.NoError(t, err)
	require.NoError(t, secondExecutor.Enqueue(3))
	if firstExecutor == secondExecutor {
		t.Fatal("expected a new queue executor after removal")
	}
}

func TestStore_StopPreventsNewQueueExecutors(t *testing.T) {
	store := newTestStore(t, NewMemory[int], func(context.Context, int) {}, WorkerPolicy{
		MinWorkers:    1,
		MaxWorkers:    1,
		JobsPerWorker: 1,
		IdleTimeout:   time.Second,
	})

	executor, err := store.Get(1)
	require.NoError(t, err)
	require.NoError(t, executor.Enqueue(1))
	store.Stop()

	_, err = store.Get(2)
	require.ErrorIs(t, err, context.Canceled)
	_, ok := store.Info(1)
	assert.False(t, ok)
	assert.NotPanics(t, store.Stop)
}

func TestStore_StopCancelsProcessorWithoutWaitingForIt(t *testing.T) {
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	finished := make(chan struct{})

	store := newTestStore(t, NewMemory[int], func(ctx context.Context, _ int) {
		started <- ctx
		<-release
		close(finished)
	}, WorkerPolicy{
		MinWorkers:    1,
		MaxWorkers:    1,
		JobsPerWorker: 1,
		IdleTimeout:   time.Second,
	})

	executor, err := store.Get(1)
	require.NoError(t, err)
	require.NoError(t, executor.Enqueue(1))

	var processorCtx context.Context
	select {
	case processorCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("processor did not start")
	}

	stopped := make(chan struct{})
	go func() {
		store.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("store waited for processor completion")
	}

	require.ErrorIs(t, processorCtx.Err(), context.Canceled)

	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("processor did not finish")
	}
}

func newTestStore[T any](
	t *testing.T,
	constructor Constructor[T],
	processor ProcessFunc[T],
	policy WorkerPolicy,
) *Store[T] {
	t.Helper()

	store, err := NewStore(constructor, processor, policy)
	require.NoError(t, err)
	return store
}

type testWorker struct {
	queue       Queue[int]
	idleTimeout time.Duration
	processed   chan<- int
}

type panicWorker struct{}

func (panicWorker) Run(context.Context) WorkerResult {
	panic("worker panic")
}

func (w *testWorker) Run(ctx context.Context) WorkerResult {
	for {
		dequeueCtx, cancel := context.WithTimeout(ctx, w.idleTimeout)
		item, err := w.queue.DequeueContext(dequeueCtx)
		cancel()

		if errors.Is(err, context.DeadlineExceeded) {
			return WorkerIdle
		}
		if err != nil {
			return WorkerStopped
		}

		w.processed <- item
	}
}
