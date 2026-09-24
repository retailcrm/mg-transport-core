package queue_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/retailcrm/mg-transport-core/v2/core/queue/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorkerPolicy() queue.WorkerPolicy {
	return queue.WorkerPolicy{
		MinWorkers: 1, MaxWorkers: 1, JobsPerWorker: 1,
		IdleTimeout: time.Second, ScaleInterval: 10 * time.Millisecond,
	}
}

func stopStore[T any](t *testing.T, store *queue.Store[T]) {
	t.Helper()
	t.Cleanup(func() { require.NoError(t, store.Stop(context.Background())) })
}

func TestStoreConstructsEachIDOnceWithoutSerializingDifferentIDs(t *testing.T) {
	var calls atomic.Int32
	started := make(chan int, 2)
	release := make(chan struct{})
	constructor := func(_ context.Context, id int) (queue.Driver[int], error) {
		calls.Add(1)
		started <- id
		<-release
		return memory.New[int](memory.Options{}), nil
	}
	store, err := queue.NewStore(constructor, func(ctx context.Context, _ int, delivery queue.Delivery[int]) {
		require.NoError(t, delivery.Ack(ctx))
	}, testWorkerPolicy())
	require.NoError(t, err)
	stopStore(t, store)

	var wg sync.WaitGroup
	for _, id := range []int{1, 2} {
		wg.Go(func() {
			_, getErr := store.Get(t.Context(), id)
			require.NoError(t, getErr)
		})
	}
	seen := map[int]bool{}
	for range 2 {
		select {
		case id := <-started:
			seen[id] = true
		case <-time.After(time.Second):
			t.Fatal("driver construction was serialized across queue IDs")
		}
	}
	close(release)
	wg.Wait()
	assert.Equal(t, map[int]bool{1: true, 2: true}, seen)

	const callers = 20
	wg = sync.WaitGroup{}
	for range callers {
		wg.Go(func() {
			_, getErr := store.Get(t.Context(), 1)
			require.NoError(t, getErr)
		})
	}
	wg.Wait()
	assert.Equal(t, int32(2), calls.Load())
}

func TestStoreScalesForJobsPublishedOutsideExecutor(t *testing.T) {
	var driver *memory.Memory[int]
	processed := make(chan int, 1)
	policy := testWorkerPolicy()
	policy.MinWorkers = 0
	store, err := queue.NewStore(func(context.Context, int) (queue.Driver[int], error) {
		driver = memory.New[int](memory.Options{})
		return driver, nil
	}, func(ctx context.Context, id int, delivery queue.Delivery[int]) {
		assert.Equal(t, 7, id)
		processed <- delivery.Value()
		require.NoError(t, delivery.Ack(ctx))
	}, policy)
	require.NoError(t, err)
	stopStore(t, store)

	_, err = store.Get(t.Context(), 7)
	require.NoError(t, err)
	require.NoError(t, driver.Enqueue(t.Context(), 42, queue.EnqueueOptions{}))
	select {
	case value := <-processed:
		assert.Equal(t, 42, value)
	case <-time.After(time.Second):
		t.Fatal("periodic scaling did not discover the driver job")
	}
}

func TestDrainLocalWaitsForCurrentProcessorAndStopsFetching(t *testing.T) {
	started := make(chan int, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProcessor := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseProcessor()
	processorDone := make(chan error, 1)
	var driver *memory.Memory[int]
	store, err := queue.NewStore(
		func(context.Context, int) (queue.Driver[int], error) {
			driver = memory.New[int](memory.Options{AckWait: time.Second})
			return driver, nil
		},
		func(ctx context.Context, _ int, delivery queue.Delivery[int]) {
			started <- delivery.Value()
			<-release
			processorDone <- ctx.Err()
			_ = delivery.Ack(ctx)
		},
		testWorkerPolicy(),
	)
	require.NoError(t, err)
	stopStore(t, store)
	require.NoError(t, store.Enqueue(t.Context(), 1, 1))
	select {
	case value := <-started:
		require.Equal(t, 1, value)
	case <-time.After(time.Second):
		t.Fatal("processor did not start")
	}

	drainCtx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	drained := make(chan error, 1)
	go func() { drained <- store.DrainLocal(drainCtx) }()
	require.Eventually(t, func() bool {
		_, getErr := store.Get(t.Context(), 1)
		return errors.Is(getErr, queue.ErrIntakeClosed)
	}, time.Second, time.Millisecond)
	select {
	case err := <-drained:
		t.Fatalf("local drain returned before the processor finished: %v", err)
	default:
	}
	releaseProcessor()
	require.NoError(t, <-drained)
	require.NoError(t, <-processorDone)
	require.NoError(t, driver.Enqueue(t.Context(), 2, queue.EnqueueOptions{}))
	info, exists, err := store.Info(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, int64(1), info.Stats.Ready)
	require.Zero(t, info.ActiveWorkers)
	select {
	case value := <-started:
		t.Fatalf("processed queued item %d after local drain", value)
	default:
	}
}

func TestStoreScalesUpAndRetiresIdleWorkers(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 6)
	policy := testWorkerPolicy()
	policy.MaxWorkers = 3
	policy.IdleTimeout = 20 * time.Millisecond
	store, err := queue.NewStore(func(context.Context, int) (queue.Driver[int], error) {
		return memory.New[int](memory.Options{}), nil
	}, func(ctx context.Context, _ int, delivery queue.Delivery[int]) {
		started <- struct{}{}
		<-release
		require.NoError(t, delivery.Ack(ctx))
	}, policy)
	require.NoError(t, err)
	stopStore(t, store)

	for value := range 6 {
		require.NoError(t, store.Enqueue(t.Context(), 1, value))
	}
	require.Eventually(t, func() bool {
		info, ok, infoErr := store.Info(t.Context(), 1)
		return infoErr == nil && ok && info.ActiveWorkers == 3
	}, time.Second, time.Millisecond)
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker did not begin processing")
		}
	}
	close(release)
	require.Eventually(t, func() bool {
		info, ok, infoErr := store.Info(t.Context(), 1)
		return infoErr == nil && ok && info.ActiveWorkers == 1
	}, time.Second, time.Millisecond)
}

func TestStoreReconcileCreatesAndRemovesExecutors(t *testing.T) {
	store, err := queue.NewStore(func(context.Context, int) (queue.Driver[int], error) {
		return memory.New[int](memory.Options{}), nil
	}, func(ctx context.Context, _ int, delivery queue.Delivery[int]) {
		require.NoError(t, delivery.Ack(ctx))
	}, testWorkerPolicy())
	require.NoError(t, err)
	stopStore(t, store)

	require.NoError(t, store.Reconcile(t.Context(), []int{1, 2, 2}))
	assert.True(t, store.Has(1))
	assert.True(t, store.Has(2))
	require.NoError(t, store.Reconcile(t.Context(), []int{2, 3}))
	assert.False(t, store.Has(1))
	assert.True(t, store.Has(2))
	assert.True(t, store.Has(3))
}

type panicWorker struct{}

func (panicWorker) Run(context.Context) queue.WorkerResult {
	panic("worker panic")
}

type waitingWorker struct{}

func (waitingWorker) Run(ctx context.Context) queue.WorkerResult {
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return queue.WorkerStopped
	case <-timer.C:
		return queue.WorkerIdle
	}
}

func TestStoreRestartsMinimumWorkerAfterWorkerPanic(t *testing.T) {
	var factoryCalls atomic.Int32
	store, err := queue.NewStore(
		func(context.Context, int) (queue.Driver[int], error) {
			return memory.New[int](memory.Options{}), nil
		},
		func(context.Context, int, queue.Delivery[int]) {},
		testWorkerPolicy(),
		queue.WithWorkerFactory(func(queue.WorkerConfig[int]) queue.Worker {
			if factoryCalls.Add(1) == 1 {
				return panicWorker{}
			}
			return waitingWorker{}
		}),
	)
	require.NoError(t, err)
	stopStore(t, store)
	_, err = store.Get(t.Context(), 1)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, ok, infoErr := store.Info(t.Context(), 1)
		return infoErr == nil && ok && info.ActiveWorkers == 1 && factoryCalls.Load() >= 2
	}, time.Second, time.Millisecond)
}

func TestStoreUsesCustomDesiredWorkerPolicy(t *testing.T) {
	policy := testWorkerPolicy()
	policy.MinWorkers = 0
	policy.MaxWorkers = 3
	policy.JobsPerWorker = 0
	policy.DesiredWorkers = func(info queue.ScaleInfo) int {
		assert.Equal(t, 9, info.ID)
		return 100
	}
	store, err := queue.NewStore(func(context.Context, int) (queue.Driver[int], error) {
		return memory.New[int](memory.Options{}), nil
	}, func(context.Context, int, queue.Delivery[int]) {}, policy)
	require.NoError(t, err)
	stopStore(t, store)
	_, err = store.Get(t.Context(), 9)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, ok, infoErr := store.Info(t.Context(), 9)
		return infoErr == nil && ok && info.ActiveWorkers == 3
	}, time.Second, time.Millisecond)
}

func TestNewStoreValidatesPolicy(t *testing.T) {
	constructor := func(context.Context, int) (queue.Driver[int], error) {
		return memory.New[int](memory.Options{}), nil
	}
	processor := func(context.Context, int, queue.Delivery[int]) {}
	_, err := queue.NewStore(constructor, processor, queue.WorkerPolicy{})
	require.EqualError(t, err, "max workers must be at least 1")
	_, err = queue.NewStore[int](nil, processor, testWorkerPolicy())
	require.EqualError(t, err, "driver constructor is required")
	_, err = queue.NewStore(constructor, nil, testWorkerPolicy())
	require.EqualError(t, err, "processor is required")
	store, err := queue.NewStore(func(context.Context, int) (queue.Driver[int], error) {
		return nil, nil
	}, processor, testWorkerPolicy())
	require.NoError(t, err)
	_, err = store.Get(t.Context(), 1)
	require.EqualError(t, err, "driver constructor returned nil driver")
}
