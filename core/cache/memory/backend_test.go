package memory_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/cache"
	"github.com/retailcrm/mg-transport-core/v2/core/cache/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriverLifecycle(t *testing.T) {
	driver, err := memory.New[string, int](memory.Options{Capacity: 10})
	require.NoError(t, err)
	c := cache.New[string, int](driver)

	value, found, err := c.Get(t.Context(), "missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Zero(t, value)

	require.NoError(t, c.Set(t.Context(), "first", 1))
	require.NoError(t, c.Set(t.Context(), "second", 2))
	require.NoError(t, c.Set(t.Context(), "first", 3))
	value, found, err = c.Get(t.Context(), "first")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 3, value)

	found, err = c.Has(t.Context(), "second")
	require.NoError(t, err)
	assert.True(t, found)
	length, err := c.Len(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, length)

	require.NoError(t, c.Delete(t.Context(), "second"))
	require.NoError(t, c.Delete(t.Context(), "missing"))
	require.NoError(t, c.Clear(t.Context()))
	length, err = c.Len(t.Context())
	require.NoError(t, err)
	assert.Zero(t, length)

	require.NoError(t, c.Close(t.Context()))
	require.NoError(t, c.Close(t.Context()))
	require.ErrorIs(t, c.Set(t.Context(), "closed", 1), cache.ErrClosed)
}

func TestDriverHonorsContext(t *testing.T) {
	driver, err := memory.New[string, int](memory.Options{Capacity: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = driver.Close(t.Context()) })

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, driver.Set(ctx, "key", 1), context.Canceled)
}

func TestDriverExpiresAfterLastSet(t *testing.T) {
	driver, err := memory.New[string, int](memory.Options{Capacity: 10, TTL: 100 * time.Millisecond})
	require.NoError(t, err)
	t.Cleanup(func() { _ = driver.Close(t.Context()) })

	require.NoError(t, driver.Set(t.Context(), "key", 1))
	time.Sleep(70 * time.Millisecond)
	require.NoError(t, driver.Set(t.Context(), "key", 2))
	time.Sleep(70 * time.Millisecond)

	value, found, err := driver.Get(t.Context(), "key")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 2, value)

	require.Eventually(t, func() bool {
		_, found, getErr := driver.Get(t.Context(), "key")
		return getErr == nil && !found
	}, time.Second, 10*time.Millisecond)
}

func TestDriverRequiresPositiveCapacity(t *testing.T) {
	_, err := memory.New[string, int](memory.Options{})
	require.Error(t, err)

	_, err = memory.New[string, int](memory.Options{Capacity: 1, TTL: -time.Second})
	require.Error(t, err)
}

func TestDriverEnforcesCapacity(t *testing.T) {
	driver, err := memory.New[int, int](memory.Options{Capacity: 2})
	require.NoError(t, err)
	t.Cleanup(func() { _ = driver.Close(t.Context()) })

	for value := range 100 {
		require.NoError(t, driver.Set(t.Context(), value, value))
	}
	require.Eventually(t, func() bool {
		length, lenErr := driver.Len(t.Context())
		return lenErr == nil && length <= 2
	}, time.Second, 10*time.Millisecond)
}

func TestDriverSupportsConcurrentAccess(t *testing.T) {
	driver, err := memory.New[string, int](memory.Options{Capacity: 1_000})
	require.NoError(t, err)
	t.Cleanup(func() { _ = driver.Close(t.Context()) })

	var workers sync.WaitGroup
	for worker := range 20 {
		workers.Go(func() {
			for value := range 100 {
				key := strconv.Itoa(worker*100 + value)
				if err := driver.Set(t.Context(), key, value); err != nil {
					t.Errorf("set %q: %v", key, err)
					return
				}
				_, _, getErr := driver.Get(t.Context(), key)
				if getErr != nil {
					t.Errorf("get %q: %v", key, getErr)
					return
				}
				if err := driver.Delete(t.Context(), key); err != nil {
					t.Errorf("delete %q: %v", key, err)
					return
				}
			}
		})
	}
	workers.Wait()
}
