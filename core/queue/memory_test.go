package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQueue(t *testing.T) {
	q, cancel := NewMemory[uint8](0)
	assert.Equal(t, int64(0), q.Len())

	q.Enqueue(1)
	q.Enqueue(2)
	assert.Equal(t, int64(2), q.Len())

	val, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, uint8(1), val)

	val, err = q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, uint8(2), val)

	ec := make(chan error)
	go func() {
		_, err = q.Dequeue()
		ec <- err
	}()
	cancel()
	select {
	case item := <-ec:
		assert.Equal(t, item, context.Canceled)
	case <-time.NewTimer(time.Millisecond).C:
		t.Fatal("timeout exceeded while waiting for context cancellation")
	}
}

func TestQueue_Concurrency(t *testing.T) {
	assert.NotPanics(t, func() {
		nq, cancel := NewMemory[int](0)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			wg.Wait()
			for i := 0; i < 1000; i++ {
				nq.Enqueue(i)
			}
		}()
		go func() {
			wg.Wait()
			for i := 1000; i < 2000; i++ {
				nq.Enqueue(i)
			}
		}()
		go func() {
			wg.Wait()
			for i := 2000; i < 3000; i++ {
				nq.Enqueue(i)
			}
		}()
		for i := 0; i < 25; i++ {
			go func() {
				wg.Wait()
				for {
					_, err := nq.Dequeue()
					if errors.Is(err, context.Canceled) {
						break
					}
				}
			}()
		}
		wg.Done()
		time.Sleep(time.Millisecond * 200)
		cancel()
	})
}

func TestQueue_FinishesWorkers(t *testing.T) {
	nq, cancel := NewMemory[int](0)
	var wg sync.WaitGroup
	stopMark := make(chan struct{})
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			for {
				_, err := nq.Dequeue()
				if err != nil && errors.Is(err, context.Canceled) {
					wg.Done()
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}
	go func() {
		wg.Wait()
		stopMark <- struct{}{}
		close(stopMark)
	}()
	for i := 1; i < 2000; i++ {
		nq.Enqueue(i)
	}
	cancel()
	select {
	case <-stopMark:
	case <-time.After(time.Second):
		t.Fatal("stopMark timeout exceeded, worker has never stopped")
	}
}
