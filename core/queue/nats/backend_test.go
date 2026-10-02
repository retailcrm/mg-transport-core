package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	corenats "github.com/retailcrm/mg-transport-core/v2/core/nats"
	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startServer(t *testing.T) *corenats.Client {
	t.Helper()
	srv, err := server.NewServer(&server.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1})
	require.NoError(t, err)
	srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)

	client, err := corenats.Connect(t.Context(), corenats.Config{URLs: []string{srv.ClientURL()}}, logger.NewNil())
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func TestDriverLifecycleAndScheduling(t *testing.T) {
	client := startServer(t)

	driver, err := New(t.Context(), client, queue.JSONCodec[string]{}, Config{
		Subject: "jobs.ready", ScheduleSubject: "jobs.schedule", Provision: Ensure,
		Stream: jetstream.StreamConfig{
			Name: "JOBS", Storage: jetstream.MemoryStorage, Retention: jetstream.WorkQueuePolicy,
		},
		Consumer:     jetstream.ConsumerConfig{Name: "workers", AckWait: 100 * time.Millisecond},
		FetchMaxWait: 20 * time.Millisecond,
	})
	require.NoError(t, err)
	q := queue.New(1, driver)
	require.NoError(t, q.Enqueue(t.Context(), "now", queue.WithID("transport-message-id")))
	require.NoError(t, q.Enqueue(t.Context(), "later", queue.WithID("scheduled id with spaces"),
		queue.WithDelay(100*time.Millisecond)))
	require.Eventually(t, func() bool {
		stats, statsErr := q.Stats(t.Context())
		return statsErr == nil && stats.Deferred == 1
	}, time.Second, 10*time.Millisecond)

	envelope, err := q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "now", envelope.Value())
	assert.Equal(t, "transport-message-id", envelope.Metadata().ID)
	require.NoError(t, envelope.Requeue(t.Context(), 20*time.Millisecond))
	envelope, err = q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "now", envelope.Value())
	assert.GreaterOrEqual(t, envelope.Metadata().Attempt, uint64(2))
	require.NoError(t, envelope.Ack(t.Context()))

	envelope, err = q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "later", envelope.Value())
	assert.Equal(t, "scheduled id with spaces", envelope.Metadata().ID)
	require.NoError(t, envelope.Reject(t.Context()))
	require.NoError(t, driver.Close(t.Context()))

	_, err = New(t.Context(), client, queue.JSONCodec[string]{}, Config{
		Subject: "jobs.ready", ScheduleSubject: "jobs.schedule", Provision: BindExisting,
		Stream: jetstream.StreamConfig{Name: "JOBS"}, Consumer: jetstream.ConsumerConfig{Name: "workers"},
	})
	require.NoError(t, err)
}

func TestRawPayloadAndDisabledScheduling(t *testing.T) {
	client := startServer(t)
	driver, err := New(t.Context(), client, queue.JSONCodec[string]{}, Config{
		Subject: "legacy.task.outbound.42", PayloadMode: PayloadRaw, DisableScheduling: true, Provision: Ensure,
		Stream: jetstream.StreamConfig{
			Name: "LEGACY_TASKS", Subjects: []string{"legacy.task.>"},
			Storage: jetstream.MemoryStorage, Retention: jetstream.WorkQueuePolicy,
		},
		Consumer: jetstream.ConsumerConfig{Name: "legacy_outbound_42"}, FetchMaxWait: 20 * time.Millisecond,
	})
	require.NoError(t, err)

	message := natsgo.NewMsg("legacy.task.outbound.42")
	message.Data = []byte(`"already queued"`)
	message.Header.Set(jetstream.MsgIDHeader, "legacy-id")
	_, err = client.JetStream.PublishMsg(t.Context(), message)
	require.NoError(t, err)

	envelope, err := driver.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "already queued", envelope.Value())
	assert.Equal(t, "legacy-id", envelope.Metadata().ID)
	assert.False(t, envelope.Metadata().EnqueuedAt.IsZero())
	require.NoError(t, envelope.Requeue(t.Context(), 10*time.Millisecond))
	envelope, err = driver.Dequeue(t.Context())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, envelope.Metadata().Attempt, uint64(2))
	require.NoError(t, envelope.Ack(t.Context()))

	q := queue.New(42, driver)
	err = q.Enqueue(t.Context(), "delayed", queue.WithDelay(time.Second))
	require.ErrorIs(t, err, queue.ErrSchedulingUnsupported)
	require.NoError(t, q.Enqueue(t.Context(), "direct", queue.WithID("direct-id")))
	envelope, err = q.Dequeue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "direct", envelope.Value())
	assert.Equal(t, "direct-id", envelope.Metadata().ID)
	require.NoError(t, envelope.Ack(t.Context()))

	stats, err := q.Stats(t.Context())
	require.NoError(t, err)
	assert.Zero(t, stats.Deferred)
}

func TestDeadLetterMalformedAndRejectedJobEnvelopes(t *testing.T) {
	client := startServer(t)
	dlqSubscription, err := client.Conn.SubscribeSync("legacy.dlq.outbound.42")
	require.NoError(t, err)
	require.NoError(t, client.Conn.Flush())

	driver, err := New(t.Context(), client, queue.JSONCodec[string]{}, Config{
		Subject: "legacy.task.outbound.42", PayloadMode: PayloadRaw, DisableScheduling: true, Provision: Ensure,
		Stream: jetstream.StreamConfig{
			Name: "LEGACY_TASKS_DLQ_TEST", Subjects: []string{"legacy.task.>"},
			Storage: jetstream.MemoryStorage, Retention: jetstream.WorkQueuePolicy,
		},
		Consumer: jetstream.ConsumerConfig{Name: "legacy_dlq_outbound_42"}, FetchMaxWait: 20 * time.Millisecond,
		DeadLetter: &DeadLetterConfig{
			Subject: "legacy.dlq.outbound.42",
			Stream: jetstream.StreamConfig{
				Name: "LEGACY_DLQ", Subjects: []string{"legacy.dlq.>"}, Storage: jetstream.MemoryStorage,
			},
		},
	})
	require.NoError(t, err)

	malformed := natsgo.NewMsg("legacy.task.outbound.42")
	malformed.Data = []byte("not JSON")
	malformed.Header.Set("Original", "header")
	_, err = client.JetStream.PublishMsg(t.Context(), malformed)
	require.NoError(t, err)
	_, err = driver.Dequeue(t.Context())
	require.ErrorContains(t, err, "decode NATS envelope")
	deadLetter := nextMessage(t, dlqSubscription)
	assert.Equal(t, malformed.Data, deadLetter.Data)
	assert.Equal(t, "header", deadLetter.Header.Get("Original"))
	assert.Equal(t, malformed.Subject, deadLetter.Header.Get("X-Original-Subject"))
	assert.Contains(t, deadLetter.Header.Get("X-Error"), "decode NATS envelope")

	q := queue.New(42, driver)
	require.NoError(t, q.Enqueue(t.Context(), "valid", queue.WithID("valid-id")))
	envelope, err := q.Dequeue(t.Context())
	require.NoError(t, err)
	cause := errors.New("permanent transport error")
	require.NoError(t, queue.DeadLetter(t.Context(), envelope, cause))
	deadLetter = nextMessage(t, dlqSubscription)
	assert.Equal(t, `"valid"`, string(deadLetter.Data))
	assert.Equal(t, cause.Error(), deadLetter.Header.Get("X-Error"))
	require.ErrorIs(t, envelope.Ack(t.Context()), queue.ErrJobEnvelopeSettled)

	_, err = New(t.Context(), client, queue.JSONCodec[string]{}, Config{
		Subject: "legacy.task.outbound.42", PayloadMode: PayloadRaw,
		DisableScheduling: true, Provision: BindExisting,
		Stream:   jetstream.StreamConfig{Name: "LEGACY_TASKS_DLQ_TEST"},
		Consumer: jetstream.ConsumerConfig{Name: "legacy_dlq_outbound_42"},
		DeadLetter: &DeadLetterConfig{
			Subject: "legacy.dlq.outbound.42",
			Stream:  jetstream.StreamConfig{Name: "LEGACY_DLQ"},
		},
	})
	require.NoError(t, err)
}

func TestFailedDeadLetterDoesNotSettleJobEnvelope(t *testing.T) {
	client := startServer(t)
	driver, err := New(t.Context(), client, queue.JSONCodec[string]{}, Config{
		Subject: "failed.task", DisableScheduling: true, Provision: Ensure,
		Stream: jetstream.StreamConfig{
			Name: "FAILED_TASK", Storage: jetstream.MemoryStorage, Retention: jetstream.WorkQueuePolicy,
		},
		Consumer: jetstream.ConsumerConfig{Name: "failed_worker"},
		DeadLetter: &DeadLetterConfig{
			Subject: "failed.dlq",
			Stream:  jetstream.StreamConfig{Name: "FAILED_DLQ", Storage: jetstream.MemoryStorage},
		},
	})
	require.NoError(t, err)
	q := queue.New(1, driver)
	require.NoError(t, q.Enqueue(t.Context(), "payload"))
	envelope, err := q.Dequeue(t.Context())
	require.NoError(t, err)

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	err = queue.DeadLetter(canceled, envelope, errors.New("failed"))
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, envelope.Settled())
	require.NoError(t, envelope.Ack(t.Context()))
}

func TestDeadLetterUnsupported(t *testing.T) {
	envelope := &unsupportedDeadLetterJobEnvelope{}
	err := queue.DeadLetter(t.Context(), envelope, errors.New("failed"))
	require.ErrorIs(t, err, queue.ErrDeadLetterUnsupported)
}

func TestCoveredByRequiresMatchingWildcardPrefix(t *testing.T) {
	assert.False(t, coveredBy([]string{"other.>"}, "queue.jobs"))
	assert.True(t, coveredBy([]string{"queue.>"}, "queue.jobs"))
	assert.False(t, coveredBy([]string{"queue.>"}, "queue"))
	assert.True(t, coveredBy([]string{"other.>", "queue.*"}, "queue.jobs"))
}

func TestDequeueStopsOnCancellationAndClose(t *testing.T) {
	client := startServer(t)
	for _, test := range []struct {
		name  string
		close bool
	}{
		{name: "context cancellation"},
		{name: "driver close", close: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver, err := New(t.Context(), client, queue.JSONCodec[string]{}, Config{
				Subject: "idle.jobs", DisableScheduling: true, Provision: Ensure,
				Stream:   jetstream.StreamConfig{Name: "IDLE_JOBS", Storage: jetstream.MemoryStorage},
				Consumer: jetstream.ConsumerConfig{Name: "idle_workers"}, FetchMaxWait: 5 * time.Second,
			})
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, dequeueErr := driver.Dequeue(ctx)
				result <- dequeueErr
			}()
			time.Sleep(50 * time.Millisecond)
			if test.close {
				require.NoError(t, driver.Close(t.Context()))
			} else {
				cancel()
			}
			select {
			case err := <-result:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("dequeue did not stop promptly")
			}
		})
	}
}

func nextMessage(t *testing.T, subscription *natsgo.Subscription) *natsgo.Msg {
	t.Helper()
	message, err := subscription.NextMsg(time.Second)
	require.NoError(t, err)
	return message
}

type unsupportedDeadLetterJobEnvelope struct{}

func (*unsupportedDeadLetterJobEnvelope) Value() string                                { return "" }
func (*unsupportedDeadLetterJobEnvelope) Metadata() queue.Metadata                     { return queue.Metadata{} }
func (*unsupportedDeadLetterJobEnvelope) Ack(context.Context) error                    { return nil }
func (*unsupportedDeadLetterJobEnvelope) Requeue(context.Context, time.Duration) error { return nil }
func (*unsupportedDeadLetterJobEnvelope) Reject(context.Context) error                 { return nil }
func (*unsupportedDeadLetterJobEnvelope) Touch(context.Context) error                  { return nil }
func (*unsupportedDeadLetterJobEnvelope) Settled() bool                                { return false }
