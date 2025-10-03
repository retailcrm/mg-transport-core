package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWorker_ProcessesItems(t *testing.T) {
	processed := make([]int, 0)
	var mu sync.Mutex

	processor := func(item int, q Queue[int]) {
		mu.Lock()
		processed = append(processed, item)
		mu.Unlock()
	}

	worker := NewWorker(processor, RecoverFuncDummy[int])
	q, cancel := NewMemory[int](1)

	go worker(q)

	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 3, len(processed))
	assert.Contains(t, processed, 1)
	assert.Contains(t, processed, 2)
	assert.Contains(t, processed, 3)
	mu.Unlock()

	cancel()
}

func TestNewWorker_ProcessesItemsInOrder(t *testing.T) {
	processed := make([]int, 0)
	var mu sync.Mutex

	processor := func(item int, q Queue[int]) {
		mu.Lock()
		processed = append(processed, item)
		mu.Unlock()
	}

	worker := NewWorker(processor, RecoverFuncDummy[int])
	q, cancel := NewMemory[int](1)

	go worker(q)

	for i := 0; i < 10; i++ {
		q.Enqueue(i)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 10, len(processed))

	for i := 0; i < 10; i++ {
		assert.Equal(t, i, processed[i])
	}
	mu.Unlock()

	cancel()
}

func TestNewWorker_StopsOnContextCancellation(t *testing.T) {
	processed := int32(0)
	stopped := make(chan bool, 1)

	processor := func(item int, q Queue[int]) {
		atomic.AddInt32(&processed, 1)
		time.Sleep(10 * time.Millisecond)
	}

	worker := NewWorker(processor, RecoverFuncDummy[int], func() {
		stopped <- true
	})

	q, cancel := NewMemory[int](1)

	go worker(q)

	for i := 0; i < 5; i++ {
		q.Enqueue(i)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-stopped:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Worker did not stop after context cancellation")
	}
}

func TestNewWorker_HandlesPanic(t *testing.T) {
	panicked := int32(0)
	processed := int32(0)

	processor := func(item int, q Queue[int]) {
		if item == 5 {
			panic("test panic")
		}
		atomic.AddInt32(&processed, 1)
	}

	recoverFunc := func(ctx context.Context, item int, r any) {
		atomic.AddInt32(&panicked, 1)
		assert.Equal(t, 5, item)
		assert.Equal(t, "test panic", r)
	}

	worker := NewWorker(processor, recoverFunc)
	q, cancel := NewMemory[int](1)

	go worker(q)

	q.Enqueue(1)
	q.Enqueue(5) // This will panic
	q.Enqueue(2)
	q.Enqueue(3)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&panicked))
	assert.Equal(t, int32(3), atomic.LoadInt32(&processed))

	cancel()
}

func TestNewWorker_ContinuesAfterPanic(t *testing.T) {
	processed := make([]int, 0)
	var mu sync.Mutex

	processor := func(item int, q Queue[int]) {
		if item%3 == 0 && item != 0 {
			panic("divisible by 3")
		}
		mu.Lock()
		processed = append(processed, item)
		mu.Unlock()
	}

	recoverFunc := func(ctx context.Context, item int, r any) {}

	worker := NewWorker(processor, recoverFunc)
	q, cancel := NewMemory[int](1)

	go worker(q)

	for i := 1; i <= 10; i++ {
		q.Enqueue(i)
	}

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 7, len(processed))
	assert.NotContains(t, processed, 3)
	assert.NotContains(t, processed, 6)
	assert.NotContains(t, processed, 9)
	mu.Unlock()

	cancel()
}

func TestNewWorker_MultipleCancelCallbacks(t *testing.T) {
	callback1Called := int32(0)
	callback2Called := int32(0)
	callback3Called := int32(0)

	processor := func(item int, q Queue[int]) {}

	worker := NewWorker(
		processor,
		RecoverFuncDummy[int],
		func() { atomic.AddInt32(&callback1Called, 1) },
		func() { atomic.AddInt32(&callback2Called, 1) },
		func() { atomic.AddInt32(&callback3Called, 1) },
	)

	q, cancel := NewMemory[int](1)
	go worker(q)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&callback1Called))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callback2Called))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callback3Called))
}

func TestNewWorker_RecoverFuncHasContext(t *testing.T) {
	var capturedCtx context.Context
	var mu sync.Mutex

	processor := func(item int, q Queue[int]) {
		panic("test")
	}

	recoverFunc := func(ctx context.Context, item int, r any) {
		mu.Lock()
		capturedCtx = ctx
		mu.Unlock()
	}

	worker := NewWorker(processor, recoverFunc)
	q, cancel := NewMemory[int](1)

	go worker(q)

	q.Enqueue(1)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.NotNil(t, capturedCtx)
	assert.Equal(t, q.Context(), capturedCtx)
	mu.Unlock()

	cancel()
}

func TestNewWorker_ConcurrentProcessing(t *testing.T) {
	processed := int32(0)

	processor := func(item int, q Queue[int]) {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&processed, 1)
	}

	q, cancel := NewMemory[int](1)

	for i := 0; i < 5; i++ {
		worker := NewWorker(processor, RecoverFuncDummy[int])
		go worker(q)
	}

	for i := 0; i < 50; i++ {
		q.Enqueue(i)
	}

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(50), atomic.LoadInt32(&processed))
	cancel()
}

func TestNewWorker_HandlesContextCanceledError(t *testing.T) {
	processor := func(item int, q Queue[int]) {}

	worker := NewWorker(processor, RecoverFuncDummy[int])
	q, cancel := NewMemory[int](1)

	done := make(chan bool, 1)
	go func() {
		worker(q)
		done <- true
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Worker did not exit on context cancellation")
	}
}

func TestDummy_ReturnsNonNilWorker(t *testing.T) {
	constructor := DummyWorker[int]()
	worker, cancel := constructor(1)

	assert.NotNil(t, worker)
	assert.NotNil(t, cancel)

	// Should be safe to call
	q, qCancel := NewMemory[int](1)
	defer qCancel()

	assert.NotPanics(t, func() {
		worker(q)
		cancel()
	})
}

func TestDummy_WorkerDoesNothing(t *testing.T) {
	constructor := DummyWorker[int]()
	worker, cancel := constructor(1)
	defer cancel()

	q, qCancel := NewMemory[int](1)
	defer qCancel()

	q.Enqueue(1)
	q.Enqueue(2)

	// Run the dummy worker
	done := make(chan bool, 1)
	go func() {
		worker(q)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Dummy worker should complete immediately")
	}

	assert.Equal(t, int64(2), q.Len())
}

func TestRecoverFuncDummy_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecoverFuncDummy[int](context.Background(), 42, "test panic")
	})
}

func TestNewWorker_ProcessorAccessesQueue(t *testing.T) {
	var capturedQueueID int
	var mu sync.Mutex

	processor := func(item int, q Queue[int]) {
		mu.Lock()
		capturedQueueID = q.ID()
		mu.Unlock()

		// Re-enqueue if less than 5
		if item < 5 {
			q.Enqueue(item + 1)
		}
	}

	worker := NewWorker(processor, RecoverFuncDummy[int])
	q, cancel := NewMemory[int](42)

	go worker(q)

	q.Enqueue(1)
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 42, capturedQueueID)
	mu.Unlock()

	cancel()
}

func TestNewWorker_ZeroValueItems(t *testing.T) {
	processed := make([]int, 0)
	var mu sync.Mutex

	processor := func(item int, q Queue[int]) {
		mu.Lock()
		processed = append(processed, item)
		mu.Unlock()
	}

	worker := NewWorker(processor, RecoverFuncDummy[int])
	q, cancel := NewMemory[int](1)

	go worker(q)

	q.Enqueue(0)
	q.Enqueue(1)
	q.Enqueue(0)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 3, len(processed))
	assert.Equal(t, 0, processed[0])
	assert.Equal(t, 1, processed[1])
	assert.Equal(t, 0, processed[2])
	mu.Unlock()

	cancel()
}

func TestNewWorker_EmptyQueue(t *testing.T) {
	processed := int32(0)

	processor := func(item int, q Queue[int]) {
		atomic.AddInt32(&processed, 1)
	}

	worker := NewWorker(processor, RecoverFuncDummy[int])
	q, cancel := NewMemory[int](1)

	go worker(q)

	// Don't enqueue anything
	time.Sleep(100 * time.Millisecond)

	// Should not have processed anything
	assert.Equal(t, int32(0), atomic.LoadInt32(&processed))

	cancel()
}
