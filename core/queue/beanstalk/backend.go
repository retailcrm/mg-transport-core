package beanstalk

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	beanstalk "github.com/beanstalkd/go-beanstalk"
	"github.com/retailcrm/mg-transport-core/v2/core/queue"
)

// Options configures a beanstalk driver.
type Options struct {
	// Priority is the beanstalkd job priority used on Put and Release; lower values mean higher priority.
	Priority uint32
	// TTR is the beanstalkd time-to-run, which acts as the delivery lease: an unsettled job is
	// re-released by the server after it expires. Non-positive values default to one minute.
	TTR time.Duration
	// PollTimeout bounds a single reserve attempt before retrying. Non-positive values default to one
	// second.
	PollTimeout time.Duration
}

// Driver is a queue.Driver implementation on top of a beanstalkd tube managed by a Manager.
type Driver[T any] struct {
	manager ManagerInterface
	codec   queue.Codec[T]
	options Options
	closed  atomic.Bool
}

type envelope struct {
	ID         string    `json:"id"`
	EnqueuedAt time.Time `json:"enqueuedAt"`
	Payload    []byte    `json:"payload"`
}

// New creates a beanstalk driver over the given manager. The codec serializes items into the
// beanstalkd job body wrapped into an envelope with the delivery ID and enqueue timestamp.
func New[T any](manager ManagerInterface, codec queue.Codec[T], options Options) *Driver[T] {
	if options.TTR <= 0 {
		options.TTR = time.Minute
	}
	if options.PollTimeout <= 0 {
		options.PollTimeout = time.Second
	}
	return &Driver[T]{manager: manager, codec: codec, options: options}
}

// Enqueue encodes the item and puts it into the tube, using native beanstalkd delays for deferred
// items.
func (b *Driver[T]) Enqueue(ctx context.Context, value T, options queue.EnqueueOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := b.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("encode beanstalk delivery: %w", err)
	}
	now := time.Now()
	id := options.ID
	if id == "" {
		id = fmt.Sprintf("beanstalk-%d", now.UnixNano())
	}
	body, err := json.Marshal(envelope{ID: id, EnqueuedAt: now, Payload: payload})
	if err != nil {
		return fmt.Errorf("encode beanstalk envelope: %w", err)
	}
	delay := max(time.Until(options.NotBefore), 0)
	if manager, ok := b.manager.(interface {
		PutContext(context.Context, []byte, uint32, time.Duration, time.Duration) (uint64, error)
	}); ok {
		_, err = manager.PutContext(ctx, body, b.options.Priority, delay, b.options.TTR)
	} else {
		_, err = b.manager.Put(body, b.options.Priority, delay, b.options.TTR)
	}
	return err
}

// Dequeue reserves the next job from the tube. Jobs whose envelope or payload cannot be decoded are
// deleted; the error is returned to the caller, and the next Dequeue attempt fetches the following job.
func (b *Driver[T]) Dequeue(ctx context.Context) (queue.Delivery[T], error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, body, err := b.manager.Reserve(b.options.PollTimeout)
		if errors.Is(err, beanstalk.ErrTimeout) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var message envelope
		if err := json.Unmarshal(body, &message); err != nil {
			_ = b.manager.Delete(id)
			return nil, fmt.Errorf("decode beanstalk envelope: %w", err)
		}
		value, err := b.codec.Decode(message.Payload)
		if err != nil {
			_ = b.manager.Delete(id)
			return nil, fmt.Errorf("decode beanstalk delivery: %w", err)
		}
		attempt, err := b.manager.Attempts(id)
		if err != nil {
			_ = b.manager.Release(id, b.options.Priority, 0)
			return nil, fmt.Errorf("read beanstalk delivery metadata: %w", err)
		}
		return &delivery[T]{driver: b, jobID: id, value: value, metadata: queue.Metadata{ID: message.ID, EnqueuedAt: message.EnqueuedAt, DeliveredAt: time.Now(), Attempt: attempt}}, nil
	}
}

// Stats maps tube statistics to the queue counters: ready, delayed, and reserved jobs.
func (b *Driver[T]) Stats(ctx context.Context) (queue.Stats, error) {
	if err := ctx.Err(); err != nil {
		return queue.Stats{}, err
	}
	stats, err := b.manager.Stats()
	return queue.Stats{Ready: stats.Ready, Deferred: stats.Delayed, InFlight: stats.Reserved}, err
}

// Close marks the driver closed and closes the underlying manager connections. Jobs remaining in the
// tube are kept by the server for later consumption.
func (b *Driver[T]) Close(context.Context) error {
	b.closed.Store(true)
	return b.manager.Close()
}

type delivery[T any] struct {
	driver   *Driver[T]
	jobID    uint64
	value    T
	metadata queue.Metadata
	settled  atomic.Bool
}

func (d *delivery[T]) Value() T {
	return d.value
}

func (d *delivery[T]) Metadata() queue.Metadata {
	return d.metadata
}

func (d *delivery[T]) Settled() bool {
	return d.settled.Load()
}
func (d *delivery[T]) terminal(operation func() error) error {
	if !d.settled.CompareAndSwap(false, true) {
		return queue.ErrDeliverySettled
	}
	if err := operation(); err != nil {
		d.settled.Store(false)
		return err
	}
	return nil
}
func (d *delivery[T]) Ack(context.Context) error {
	return d.terminal(func() error { return d.driver.manager.Delete(d.jobID) })
}
func (d *delivery[T]) Reject(ctx context.Context) error {
	return d.Ack(ctx)
}
func (d *delivery[T]) Requeue(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.terminal(func() error { return d.driver.manager.Release(d.jobID, d.driver.options.Priority, delay) })
}
func (d *delivery[T]) Touch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.Settled() {
		return queue.ErrDeliverySettled
	}
	return d.driver.manager.Touch(d.jobID)
}

var _ queue.Driver[int] = (*Driver[int])(nil)
