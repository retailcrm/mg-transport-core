package queue

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Memory represents a thread-safe FIFO in-memory queue.
type Memory[T any] struct {
	id           int
	items        *list.List
	mutex        sync.Mutex
	cond         *sync.Cond
	size         int64 // atomic counter for size
	ctx          context.Context
	cancel       context.CancelFunc
	lastEnqueued atomic.Value // when the last item was added
}

// Queue is a queue interface.
type Queue[T any] interface {
	Info
	// Enqueue puts an item into queue, returns context.Cancelled when queue is stopped.
	Enqueue(T) error
	// Dequeue item from queue. This method should return leftover enqueued items even if queue was cancelled.
	Dequeue() (T, error)
}

// Info is a queue information interface.
type Info interface {
	// ID of the queue.
	ID() int
	// Context returns queue context.Context.
	Context() context.Context
	// LastEnqueueTime returns last Enqueue call time.
	LastEnqueueTime() time.Time
	// Len returns the amount of items in the queue.
	Len() int64
}

// NewMemory creates a new Memory queue with context.
func NewMemory[T any](id int) (Queue[T], context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	q := &Memory[T]{
		id:     id,
		items:  list.New(),
		ctx:    ctx,
		cancel: cancel,
	}
	q.cond = sync.NewCond(&q.mutex)
	return q, q.stop
}

// Enqueue adds an item to the end of the queue
func (q *Memory[T]) Enqueue(item T) error {
	if err := q.ctx.Err(); err != nil {
		return err
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.items.PushBack(item)
	atomic.AddInt64(&q.size, 1)
	q.lastEnqueued.Store(time.Now())
	q.cond.Signal() // Signal one waiting goroutine
	return nil
}

// Dequeue an item from the queue start.
func (q *Memory[T]) Dequeue() (T, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for q.items.Len() == 0 {
		if err := q.ctx.Err(); err != nil {
			var zero T
			return zero, err
		}
		q.cond.Wait()
	}

	item := q.items.Remove(q.items.Front()).(T)
	atomic.AddInt64(&q.size, -1)
	return item, nil
}

// ID returns the current queue ID
func (q *Memory[T]) ID() int {
	return q.id
}

// Context returns the current queue context
func (q *Memory[T]) Context() context.Context {
	return q.ctx
}

// LastEnqueueTime returns unix time in nanoseconds for the last Enqueue call.
func (q *Memory[T]) LastEnqueueTime() time.Time {
	val := q.lastEnqueued.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// Len returns the current size of the queue
func (q *Memory[T]) Len() int64 {
	return atomic.LoadInt64(&q.size)
}

// stop cancels the context and broadcasts to all waiting goroutines which wakes them up (they terminate themselves).
func (q *Memory[T]) stop() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.cancel()
	q.cond.Broadcast()
}
