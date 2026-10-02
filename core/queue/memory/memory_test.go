package memory

import (
	"context"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/require"
)

func TestDequeueReadyItemAfterCancellationOrClose(t *testing.T) {
	for _, test := range []struct {
		name   string
		cancel bool
	}{
		{name: "canceled context", cancel: true},
		{name: "closed driver"},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := New[int](Options{})
			require.NoError(t, driver.Enqueue(t.Context(), 42, queue.EnqueueOptions{}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.cancel {
				cancel()
			} else {
				require.NoError(t, driver.Close(t.Context()))
			}
			envelope, err := driver.Dequeue(ctx)
			require.Nil(t, envelope)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestLongRunningProcessorKeepsMemoryJobEnvelope(t *testing.T) {
	driver := New[int](Options{AckWait: 30 * time.Millisecond})
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	store, err := queue.NewStore(
		func(context.Context, int) (queue.Driver[int], error) { return driver, nil },
		func(ctx context.Context, _ int, envelope queue.JobEnvelope[int]) {
			started <- struct{}{}
			<-release
			_ = envelope.Ack(ctx)
		},
		queue.WorkerPolicy{
			MinWorkers: 2, MaxWorkers: 2, JobsPerWorker: 1,
			IdleTimeout: time.Second, ScaleInterval: 10 * time.Millisecond,
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = store.Stop(ctx)
	})
	require.NoError(t, store.Enqueue(t.Context(), 1, 42))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("processor did not start")
	}
	select {
	case <-started:
		t.Fatal("envelope was processed twice before the first processor finished")
	case <-time.After(120 * time.Millisecond):
	}
}
