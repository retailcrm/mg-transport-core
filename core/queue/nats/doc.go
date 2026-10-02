// Package nats provides a queue.Driver implementation backed by NATS JetStream.
//
// # Architecture
//
// The driver uses one JetStream stream and one durable pull consumer. Ready items are published to
// the queue subject and delivered through the consumer with explicit acknowledgments. Deferred items
// (WithDelay/WithNotBefore) normally rely on JetStream message schedules: the item is published under
// "<ScheduleSubject>.<token>" with a schedule-at time and a target pointing at the queue subject, so
// the server itself moves the item into place when it becomes due. This requires the stream to be
// created with AllowMsgSchedules enabled; the driver refuses to bind to a stream without it. Set
// DisableScheduling to bind a legacy stream without this feature; delayed negative acknowledgments
// still work, while delayed enqueue returns queue.ErrSchedulingUnsupported.
//
// By default, items are wrapped into an envelope (job ID, enqueue timestamp, encoded payload).
// PayloadRaw stores only the queue.Codec output and derives metadata from the NATS header and stream
// metadata, allowing a driver to consume messages from legacy direct publishers. The caller-provided
// enqueue ID doubles as the JetStream message ID, giving
// publisher-side deduplication for free. Job envelope leases map to the consumer acknowledgment wait:
// Touch sends in-progress working acknowledgments, Requeue maps to a negative acknowledgment with
// delay, and Reject terminates the message. An optional DeadLetter configuration preserves malformed
// messages automatically; processors can call queue.DeadLetter to preserve terminal failures with a
// cause before terminating them.
//
// The Provision mode selects how the stream and consumer are obtained: Ensure creates or updates both
// (also covering the queue and schedule subjects), while BindExisting only binds to pre-provisioned
// resources and validates their configuration. BindExisting is intended for environments where
// infrastructure is managed externally.
//
// # Usage
//
//	client, err := corenats.Connect(ctx, corenats.Config{URLs: []string{"nats://localhost:4222"}}, log)
//	if err != nil {
//	    return err
//	}
//	driver, err := nats.New[Job](ctx, client, queue.JSONCodec[Job]{}, nats.Config{
//	    Subject:   "transport.jobs",
//	    Stream:    jetstream.StreamConfig{Name: "TRANSPORT", AllowMsgSchedules: true},
//	    Consumer:  jetstream.ConsumerConfig{Name: "transport-jobs"},
//	    Provision: nats.Ensure,
//	})
//
// The driver is durable: undelivered and unsettled items survive restarts inside JetStream.
package nats
