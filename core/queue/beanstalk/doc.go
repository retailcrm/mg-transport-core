// Package beanstalk provides a queue.Driver implementation backed by beanstalkd tubes.
//
// # Architecture
//
// A Manager owns two dedicated TCP connections to the beanstalkd server (beanstalkd allows a single
// in-flight reserve per connection): a producer connection bound to the tube for Put operations and a
// consumer connection bound to a tube set for Reserve and settlement operations. Both connections
// transparently reconnect with a configurable delay on network errors, and Close shuts both down.
//
// The Driver adapts the Manager to the queue.Driver contract. Items are wrapped into an envelope
// carrying the caller-provided ID and the enqueue timestamp, serialized with a queue.Codec. Envelope
// leases map to the beanstalkd time-to-run: Touch renews the lease, Requeue maps to Release with a
// delay, Ack and Reject both Delete the job, and a lease expiry re-releases the job server-side.
//
// # Usage
//
//	manager, err := beanstalk.NewManager(ctx, "beanstalkd:11300", "transport.jobs", log, time.Second)
//	if err != nil {
//	    return err
//	}
//	driver := beanstalk.New[Job](manager, queue.JSONCodec[Job]{}, beanstalk.Options{
//	    Priority: 1,
//	    TTR:      time.Minute,
//	})
//
// The driver is durable across restarts: jobs live in beanstalkd until deleted, and delayed items use
// native beanstalkd delays. When reusing a tube containing bodies written by the old queue, pass
// NewLegacyBodyAdapter(manager) to New so old jobs remain consumable during the migration.
package beanstalk
