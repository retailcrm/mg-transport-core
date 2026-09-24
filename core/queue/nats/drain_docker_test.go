package nats

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	corenats "github.com/retailcrm/mg-transport-core/v2/core/nats"
	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/require"
)

// NATS_TEST_URL=nats://127.0.0.1:<port> go test ./core/queue/nats -run TestDrainSeesOtherReplicaDelivery -count=1
func TestDrainSeesOtherReplicaDelivery(t *testing.T) {
	url := os.Getenv("NATS_TEST_URL")
	if url == "" {
		t.Skip("set NATS_TEST_URL to test against NATS in Docker")
	}
	localClient, err := corenats.Connect(t.Context(), corenats.Config{URLs: []string{url}}, logger.NewNil())
	require.NoError(t, err)
	t.Cleanup(localClient.Close)
	remoteClient, err := corenats.Connect(t.Context(), corenats.Config{URLs: []string{url}}, logger.NewNil())
	require.NoError(t, err)
	t.Cleanup(remoteClient.Close)

	suffix := time.Now().UnixNano()
	config := Config{
		Subject: fmt.Sprintf("drain.%d.jobs", suffix), DisableScheduling: true, Provision: Ensure,
		Stream: jetstream.StreamConfig{
			Name: fmt.Sprintf("DRAIN_%d", suffix), Storage: jetstream.MemoryStorage,
			Retention: jetstream.WorkQueuePolicy,
		},
		Consumer: jetstream.ConsumerConfig{Name: "shared_workers", AckWait: time.Second},
	}
	local, err := queue.NewStore(
		func(ctx context.Context, _ int) (queue.Driver[string], error) {
			return New(ctx, localClient, queue.JSONCodec[string]{}, config)
		},
		func(context.Context, int, queue.Delivery[string]) {},
		queue.WorkerPolicy{
			MinWorkers: 0, MaxWorkers: 1, IdleTimeout: time.Second,
			ScaleInterval: time.Second, DesiredWorkers: func(queue.ScaleInfo) int { return 0 },
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = local.Stop(context.Background()) })
	_, err = local.Get(t.Context(), 1)
	require.NoError(t, err)

	config.Provision = BindExisting
	remote, err := New(t.Context(), remoteClient, queue.JSONCodec[string]{}, config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = remote.Close(context.Background()) })
	require.NoError(t, remote.Enqueue(t.Context(), "other replica's job", queue.EnqueueOptions{}))
	delivery, err := remote.Dequeue(t.Context())
	require.NoError(t, err)

	local.CloseIntake()
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, local.Drain(ctx), context.DeadlineExceeded)
	localCtx, localCancel := context.WithTimeout(t.Context(), time.Second)
	defer localCancel()
	require.NoError(t, local.DrainLocal(localCtx))
	require.NoError(t, delivery.Ack(t.Context()))
	finished, done := context.WithTimeout(t.Context(), time.Second)
	defer done()
	require.NoError(t, local.Drain(finished))
}
