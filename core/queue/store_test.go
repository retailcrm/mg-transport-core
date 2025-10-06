package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_GetCreatesQueue(t *testing.T) {
	store := NewStore(NewMemory[int])
	q := store.Get(1)
	assert.NotNil(t, q)
	assert.Equal(t, 1, q.ID())
}

func TestStore_GetReturnsSameQueue(t *testing.T) {
	store := NewStore(NewMemory[int])
	q1 := store.Get(1)
	q2 := store.Get(1)
	assert.Equal(t, q1, q2)
}

func TestStore_GetDifferentQueues(t *testing.T) {
	store := NewStore(NewMemory[int])
	q1 := store.Get(1)
	q2 := store.Get(2)
	assert.NotEqual(t, q1, q2)
	assert.Equal(t, 1, q1.ID())
	assert.Equal(t, 2, q2.ID())
}

func TestStore_RemoveStopsQueue(t *testing.T) {
	store := NewStore(NewMemory[int])
	q := store.Get(1)

	q.Enqueue(1)
	q.Enqueue(2)

	store.Remove(1)

	require.ErrorIs(t, q.Enqueue(3), context.Canceled)

	dequeueReturns := func(q Queue[int], val int) {
		actual, err := q.Dequeue()
		require.NoError(t, err)
		assert.Equal(t, val, actual)
	}

	dequeueReturns(q, 1)
	dequeueReturns(q, 2)

	_, err := q.Dequeue()
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestStore_WithNumWorkers(t *testing.T) {
	processed := int32(0)
	workerCount := int32(0)

	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		atomic.AddInt32(&workerCount, 1)
		worker := NewWorker(
			func(_ int, _ Queue[int]) {
				atomic.AddInt32(&processed, 1)
			},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(3)

	q := store.Get(1)

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(3), atomic.LoadInt32(&workerCount))

	// Enqueue items and verify they're processed
	for i := 0; i < 10; i++ {
		q.Enqueue(i)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(10), atomic.LoadInt32(&processed))

	store.Remove(1)
}

func TestStore_WithWorkerConstructor(t *testing.T) {
	processed := make([]int, 0)
	var mu sync.Mutex

	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(item int, _ Queue[int]) {
				mu.Lock()
				processed = append(processed, item)
				mu.Unlock()
			},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor)

	q := store.Get(1)
	q.Enqueue(100)
	q.Enqueue(200)
	q.Enqueue(300)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Len(t, processed, 3)
	assert.Contains(t, processed, 100)
	assert.Contains(t, processed, 200)
	assert.Contains(t, processed, 300)
	mu.Unlock()

	store.Remove(1)
}

func TestStore_WithMaxNumWorkers(t *testing.T) {
	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(_ int, _ Queue[int]) {
				time.Sleep(10 * time.Millisecond)
			},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(2).
		WithMaxNumWorkers(5)

	q := store.Get(1)
	assert.NotNil(t, q)

	// Verify initial workers are started
	time.Sleep(50 * time.Millisecond)
	left, active := store.scalingInfo(1)
	assert.Equal(t, 3, left) // 5 max - 2 active = 3 left
	assert.Equal(t, 2, active)

	store.Remove(1)
}

func TestStore_WithScaleFunc(t *testing.T) {
	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(_ int, _ Queue[int]) {
				time.Sleep(50 * time.Millisecond)
			},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	addWorkerCalled := int32(0)
	deleteWorkerCalled := int32(0)

	scaleFunc := func(q Info, addWorker, deleteWorker func(), availableScaling func() (int, int)) {
		qLen := q.Len()
		slotsLeft, slotsActive := availableScaling()

		if qLen > 10 && slotsLeft > 0 {
			atomic.AddInt32(&addWorkerCalled, 1)
			addWorker()
		} else if qLen < 2 && slotsActive > 1 {
			atomic.AddInt32(&deleteWorkerCalled, 1)
			deleteWorker()
		}
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(1).
		WithMaxNumWorkers(3).
		WithScaleFunc(scaleFunc, 50*time.Millisecond)

	q := store.Get(1)

	// Enqueue many items to trigger scale up
	for i := 0; i < 20; i++ {
		q.Enqueue(i)
	}

	// Wait for scale function to run
	time.Sleep(200 * time.Millisecond)

	assert.Positive(t, atomic.LoadInt32(&addWorkerCalled))

	store.Remove(1)
}

func TestStore_AddWorkerRespectMaxLimit(t *testing.T) {
	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(_ int, _ Queue[int]) {},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(2).
		WithMaxNumWorkers(3)

	q := store.Get(1)
	assert.NotNil(t, q)
	time.Sleep(50 * time.Millisecond)

	// Try to add worker - should succeed (2 -> 3)
	store.addWorker(1)
	time.Sleep(50 * time.Millisecond)
	left, active := store.scalingInfo(1)
	assert.Equal(t, 0, left)
	assert.Equal(t, 3, active)

	// Try to add another worker - should fail (at max)
	store.addWorker(1)
	time.Sleep(50 * time.Millisecond)
	left, active = store.scalingInfo(1)
	assert.Equal(t, 0, left)
	assert.Equal(t, 3, active)

	store.Remove(1)
}

func TestStore_StopWorkerRespectMinLimit(t *testing.T) {
	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(_ int, _ Queue[int]) {},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(2).
		WithMaxNumWorkers(4)

	q := store.Get(1)
	assert.NotNil(t, q)
	time.Sleep(50 * time.Millisecond)

	// Stop one worker (2 -> 1)
	store.stopWorker(1)
	time.Sleep(50 * time.Millisecond)
	_, active := store.scalingInfo(1)
	assert.Equal(t, 1, active)

	// Try to stop another worker - should fail (can't go below 1)
	store.stopWorker(1)
	time.Sleep(50 * time.Millisecond)
	_, active = store.scalingInfo(1)
	assert.Equal(t, 1, active)

	store.Remove(1)
}

func TestStore_ConcurrentAccess(t *testing.T) {
	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(_ int, _ Queue[int]) {},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(1)

	var wg sync.WaitGroup

	// Concurrent Get operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			q := store.Get(id)
			q.Enqueue(id)
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// Verify all queues exist
	for i := 0; i < 10; i++ {
		q := store.Get(i)
		assert.Equal(t, i, q.ID())
		store.Remove(i)
	}
}

func TestStore_ScaleFuncPanicRecovery(t *testing.T) {
	workerConstructor := func(_ int) (Worker[int], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(_ int, _ Queue[int]) {},
			RecoverFuncDummy[int],
		)
		return worker, cancel
	}

	panicCount := int32(0)
	recovered := make(chan bool, 1)

	scaleFunc := func(_ Info, _, _ func(), _ func() (int, int)) {
		count := atomic.AddInt32(&panicCount, 1)
		if count == 1 {
			panic("test panic")
		}
		if count > 1 {
			recovered <- true
		}
	}

	store := NewStore(NewMemory[int]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(1).
		WithScaleFunc(scaleFunc, 50*time.Millisecond)

	store.Get(1)

	// Wait for panic and recovery
	select {
	case <-recovered:
		// Successfully recovered
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Scale function did not recover from panic")
	}

	store.Remove(1)
}

func TestStore_MultipleQueuesIndependentWorkers(t *testing.T) {
	processed := sync.Map{}

	workerConstructor := func(_ int) (Worker[func() (int, func())], context.CancelFunc) {
		_, cancel := context.WithCancel(context.Background())
		worker := NewWorker(
			func(item func() (int, func()), q Queue[func() (int, func())]) {
				num, finish := item()
				key := q.ID()
				val, _ := processed.LoadOrStore(key, &[]int{})
				list := val.(*[]int)
				*list = append(*list, num)
				finish()
			},
			RecoverFuncDummy[func() (int, func())],
		)
		return worker, cancel
	}

	store := NewStore(NewMemory[func() (int, func())]).
		WithWorkerConstructor(workerConstructor).
		WithNumWorkers(1)

	q1 := store.Get(1)
	q2 := store.Get(2)

	var wg sync.WaitGroup
	buildItem := func(val int) func() (int, func()) {
		return func() (int, func()) {
			return val, wg.Done
		}
	}

	wg.Add(4)

	q1.Enqueue(buildItem(10))
	q1.Enqueue(buildItem(20))
	q2.Enqueue(buildItem(30))
	q2.Enqueue(buildItem(40))

	wg.Wait()

	val1, _ := processed.Load(1)
	val2, _ := processed.Load(2)

	list1 := val1.(*[]int)
	list2 := val2.(*[]int)

	assert.Len(t, *list1, 2)
	assert.Len(t, *list2, 2)
	assert.Contains(t, *list1, 10)
	assert.Contains(t, *list1, 20)
	assert.Contains(t, *list2, 30)
	assert.Contains(t, *list2, 40)

	store.Remove(1)
	store.Remove(2)
}
