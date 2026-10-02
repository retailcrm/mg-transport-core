// Package memory provides a process-local queue.Driver implementation.
//
// # Architecture
//
// The driver keeps three collections guarded by a mutex: a FIFO slice of ready items, a heap of
// deferred items ordered by their NotBefore time, and a map of in-flight envelopes. Enqueue appends
// to the ready slice or pushes onto the deferred heap depending on the delay options. Dequeue promotes
// due deferred items, hands out the oldest ready item, and arms a per-envelope lease timer; when the
// lease (AckWait) expires without settlement, the envelope is returned to the ready slice so another
// worker can pick it up. A notification channel wakes blocked dequeues as soon as new work arrives,
// which keeps polling cost near zero while the queue is empty.
//
// Because state lives only in the process, enqueued items are lost on restart. The driver is a good
// fit for tests, single-instance deployments, and work that can be re-created from a durable source.
//
// # Usage
//
//	driver := memory.New[Job](memory.Options{AckWait: 30 * time.Second})
//	jobs, err := queue.NewStore(
//	    func(context.Context, int) (queue.Driver[Job], error) { return driver, nil },
//	    processor,
//	    policy,
//	)
//
// Options with an empty or non-positive AckWait default to 30 seconds.
package memory
