package memory

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
)

// Options configures a memory driver.
type Options struct {
	// AckWait is the envelope lease duration: an unsettled envelope is re-queued after it expires.
	// Non-positive values default to 30 seconds.
	AckWait time.Duration
}

type item[T any] struct {
	internalID string
	id         string
	value      T
	enqueuedAt time.Time
	notBefore  time.Time
	attempt    uint64
	index      int
}

type delayedHeap[T any] []*item[T]

func (h delayedHeap[T]) Len() int {
	return len(h)
}

func (h delayedHeap[T]) Less(i, j int) bool {
	return h[i].notBefore.Before(h[j].notBefore)
}

func (h delayedHeap[T]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

func (h *delayedHeap[T]) Push(value any) {
	*h = append(*h, value.(*item[T]))
}
func (h *delayedHeap[T]) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}

// Memory is a process-local Driver with ready, deferred, and in-flight item sets guarded by a mutex.
// It implements queue.Driver and is safe for concurrent use within one process; state does not
// survive restarts.
type Memory[T any] struct {
	mu        sync.Mutex
	notify    chan struct{}
	closedCh  chan struct{}
	ready     []*item[T]
	delayed   delayedHeap[T]
	inFlight  map[string]*memoryJobEnvelope[T]
	ackWait   time.Duration
	closed    bool
	closeOnce sync.Once
	sequence  atomic.Uint64
}

// New creates a memory driver. Items are held only in the current process, and delayed items are
// scheduled internally with a heap ordered by their NotBefore time.
func New[T any](options Options) *Memory[T] {
	if options.AckWait <= 0 {
		options.AckWait = 30 * time.Second
	}
	b := &Memory[T]{notify: make(chan struct{}, 1), closedCh: make(chan struct{}), ackWait: options.AckWait, inFlight: make(map[string]*memoryJobEnvelope[T])}
	heap.Init(&b.delayed)
	return b
}

func (b *Memory[T]) signal() {
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// Enqueue stores the item in the ready set or, when delayed options are given, in the deferred heap.
func (b *Memory[T]) Enqueue(ctx context.Context, value T, options queue.EnqueueOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	internalID := fmt.Sprintf("memory-%d", b.sequence.Add(1))
	id := options.ID
	if id == "" {
		id = internalID
	}
	entry := &item[T]{internalID: internalID, id: id, value: value, enqueuedAt: now, notBefore: options.NotBefore}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return context.Canceled
	}
	if !entry.notBefore.IsZero() && entry.notBefore.After(now) {
		heap.Push(&b.delayed, entry)
	} else {
		b.ready = append(b.ready, entry)
	}
	b.signal()
	return nil
}

func (b *Memory[T]) promote(now time.Time) {
	for len(b.delayed) > 0 && !b.delayed[0].notBefore.After(now) {
		entry := heap.Pop(&b.delayed).(*item[T])
		b.ready = append(b.ready, entry)
	}
}

func (b *Memory[T]) nextDelay(now time.Time) time.Duration {
	if len(b.delayed) == 0 {
		return time.Hour
	}
	return max(time.Until(b.delayed[0].notBefore), time.Millisecond)
}

// Dequeue waits for the next ready item, promoting due deferred items first. The returned envelope
// carries an AckWait lease; an expired unsettled envelope is re-queued automatically.
func (b *Memory[T]) Dequeue(ctx context.Context) (queue.JobEnvelope[T], error) {
	for {
		b.mu.Lock()
		if err := ctx.Err(); err != nil {
			b.mu.Unlock()
			return nil, err
		}
		if b.closed {
			b.mu.Unlock()
			return nil, context.Canceled
		}
		now := time.Now()
		b.promote(now)
		if len(b.ready) > 0 {
			entry := b.ready[0]
			b.ready = b.ready[1:]
			entry.attempt++
			envelope := &memoryJobEnvelope[T]{driver: b, entry: entry, deliveredAt: now}
			initialized := make(chan struct{})
			envelope.timer = time.AfterFunc(b.ackWait, func() {
				<-initialized
				envelope.expire()
			})
			close(initialized)
			b.inFlight[entry.internalID] = envelope
			b.mu.Unlock()
			return envelope, nil
		}
		wait := b.nextDelay(now)
		b.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-b.notify:
			if !timer.Stop() {
				<-timer.C
			}
		case <-b.closedCh:
			if !timer.Stop() {
				<-timer.C
			}
			return nil, context.Canceled
		case <-timer.C:
		}
	}
}

// Stats returns the sizes of the ready, deferred, and in-flight sets.
func (b *Memory[T]) Stats(ctx context.Context) (queue.Stats, error) {
	if err := ctx.Err(); err != nil {
		return queue.Stats{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.promote(time.Now())
	return queue.Stats{Ready: int64(len(b.ready)), Deferred: int64(len(b.delayed)), InFlight: int64(len(b.inFlight))}, nil
}

// Close stops the lease timers and wakes all blocked dequeues with context.Canceled. Enqueued but
// unprocessed items are discarded. Close is idempotent.
func (b *Memory[T]) Close(context.Context) error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		for _, envelope := range b.inFlight {
			envelope.timer.Stop()
		}
		b.mu.Unlock()
		close(b.closedCh)
	})
	return nil
}

type memoryJobEnvelope[T any] struct {
	driver      *Memory[T]
	entry       *item[T]
	deliveredAt time.Time
	timer       *time.Timer
	settled     atomic.Bool
}

func (d *memoryJobEnvelope[T]) Value() T {
	return d.entry.value
}
func (d *memoryJobEnvelope[T]) Metadata() queue.Metadata {
	return queue.Metadata{ID: d.entry.id, EnqueuedAt: d.entry.enqueuedAt, DeliveredAt: d.deliveredAt, Attempt: d.entry.attempt}
}
func (d *memoryJobEnvelope[T]) Settled() bool {
	return d.settled.Load()
}

// AutoRenewInterval keeps a managed processor's lease alive while it is running.
func (d *memoryJobEnvelope[T]) AutoRenewInterval() time.Duration {
	return max(d.driver.ackWait/3, time.Nanosecond)
}

func (d *memoryJobEnvelope[T]) terminal(fn func()) error {
	if !d.settled.CompareAndSwap(false, true) {
		return queue.ErrJobEnvelopeSettled
	}
	d.timer.Stop()
	d.driver.mu.Lock()
	delete(d.driver.inFlight, d.entry.internalID)
	fn()
	d.driver.mu.Unlock()
	d.driver.signal()
	return nil
}

func (d *memoryJobEnvelope[T]) Ack(context.Context) error {
	return d.terminal(func() {})
}

func (d *memoryJobEnvelope[T]) Reject(context.Context) error {
	return d.terminal(func() {})
}
func (d *memoryJobEnvelope[T]) Requeue(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.terminal(func() {
		d.entry.notBefore = time.Now().Add(delay)
		if delay > 0 {
			heap.Push(&d.driver.delayed, d.entry)
		} else {
			d.driver.ready = append(d.driver.ready, d.entry)
		}
	})
}
func (d *memoryJobEnvelope[T]) Touch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.driver.mu.Lock()
	defer d.driver.mu.Unlock()
	if d.settled.Load() {
		return queue.ErrJobEnvelopeSettled
	}
	if d.driver.closed {
		return context.Canceled
	}
	d.timer.Reset(d.driver.ackWait)
	return nil
}
func (d *memoryJobEnvelope[T]) expire() {
	_ = d.terminal(func() { d.driver.ready = append(d.driver.ready, d.entry) })
}

var _ queue.Driver[int] = (*Memory[int])(nil)
