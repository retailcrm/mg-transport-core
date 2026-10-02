package queue_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/retailcrm/mg-transport-core/v2/core/queue/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueJobEnvelopeLifecycle(t *testing.T) {
	driver := memory.New[int](memory.Options{AckWait: 50 * time.Millisecond})
	q := queue.New(7, driver)
	require.NoError(t, q.Enqueue(t.Context(), 1, queue.WithID("caller-id")))
	require.NoError(t, q.Enqueue(t.Context(), 2, queue.WithDelay(30*time.Millisecond)))

	stats, err := q.Stats(t.Context())
	require.NoError(t, err)
	assert.Equal(t, queue.Stats{Ready: 1, Deferred: 1}, stats)

	envelope, err := q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, envelope.Value())
	assert.Equal(t, "caller-id", envelope.Metadata().ID)
	assert.Equal(t, uint64(1), envelope.Metadata().Attempt)
	require.NoError(t, envelope.Requeue(t.Context(), 0))
	require.ErrorIs(t, envelope.Ack(t.Context()), queue.ErrJobEnvelopeSettled)

	envelope, err = q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(2), envelope.Metadata().Attempt)
	require.NoError(t, envelope.Touch(t.Context()))
	require.NoError(t, envelope.Ack(t.Context()))

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	envelope, err = q.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, envelope.Value())
	require.NoError(t, envelope.Reject(t.Context()))
}

func TestMemoryRedeliversExpiredJobEnvelope(t *testing.T) {
	q := queue.New(1, memory.New[string](memory.Options{AckWait: 20 * time.Millisecond}))
	require.NoError(t, q.Enqueue(t.Context(), "job"))
	first, err := q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.False(t, first.Settled())

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	second, err := q.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, "job", second.Value())
	assert.Equal(t, uint64(2), second.Metadata().Attempt)
	require.NoError(t, second.Ack(t.Context()))
}

func TestWorkerUsesUnsettledProcessor(t *testing.T) {
	var called atomic.Int64
	store, err := queue.NewStore(
		func(context.Context, int) (queue.Driver[int], error) {
			return memory.New[int](memory.Options{}), nil
		},
		func(context.Context, int, queue.JobEnvelope[int]) {},
		queue.WorkerPolicy{MinWorkers: 1, MaxWorkers: 1, JobsPerWorker: 1, IdleTimeout: time.Second, ScaleInterval: time.Second},
		queue.WithUnsettledProcessor(func(ctx context.Context, _ int, envelope queue.JobEnvelope[int], cause queue.UnsettledCause) {
			called.Add(1)
			assert.Equal(t, queue.UnsettledReturned, cause.Kind)
			require.NoError(t, envelope.Ack(ctx))
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Stop(context.Background())) })
	require.NoError(t, store.Enqueue(t.Context(), 1, 42))
	require.Eventually(t, func() bool { return called.Load() == 1 }, time.Second, time.Millisecond)
}

func TestStoreConstructsAndDrainsQueues(t *testing.T) {
	store, err := queue.NewStore(
		func(context.Context, int) (queue.Driver[int], error) {
			return memory.New[int](memory.Options{}), nil
		},
		func(ctx context.Context, _ int, envelope queue.JobEnvelope[int]) {
			require.NoError(t, envelope.Ack(ctx))
		},
		queue.WorkerPolicy{MinWorkers: 1, MaxWorkers: 1, JobsPerWorker: 1, IdleTimeout: time.Second, ScaleInterval: time.Second},
	)
	require.NoError(t, err)
	q, err := store.Get(t.Context(), 5)
	require.NoError(t, err)
	require.NoError(t, q.Enqueue(t.Context(), 1))
	store.CloseIntake()
	require.ErrorIs(t, q.Enqueue(t.Context(), 2), queue.ErrIntakeClosed)
	require.NoError(t, store.Drain(t.Context()))
	require.NoError(t, store.Stop(t.Context()))
}
