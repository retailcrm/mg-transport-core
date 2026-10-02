package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/retailcrm/mg-transport-core/v2/core/cache"
)

// Options configures a memory driver. Capacity is required; TTL is optional.
type Options struct {
	// Capacity is the maximum number of entries; it must be positive.
	Capacity int
	// TTL expires every entry a fixed duration after it was written. Zero disables expiry.
	TTL time.Duration
}

// Driver is a process-local cache.Driver built on an otter cache with hard capacity and an optional
// write-time TTL. It is safe for concurrent use and does not persist entries.
type Driver[K comparable, V any] struct {
	cache     *otter.Cache[K, V]
	closed    atomic.Bool
	closeOnce sync.Once
}

// New creates a memory driver with the given capacity and TTL. It fails when the capacity is not
// positive or the TTL is negative.
func New[K comparable, V any](options Options) (*Driver[K, V], error) {
	if options.Capacity <= 0 {
		return nil, errors.New("memory cache capacity must be positive")
	}
	if options.TTL < 0 {
		return nil, errors.New("memory cache TTL must not be negative")
	}

	otterOptions := &otter.Options[K, V]{MaximumSize: options.Capacity}
	if options.TTL > 0 {
		otterOptions.ExpiryCalculator = otter.ExpiryWriting[K, V](options.TTL)
	}
	storage, err := otter.New(otterOptions)
	if err != nil {
		return nil, fmt.Errorf("create memory cache: %w", err)
	}
	return &Driver[K, V]{cache: storage}, nil
}

func (b *Driver[K, V]) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.closed.Load() {
		return cache.ErrClosed
	}
	return nil
}

// Get returns the value for the key if present and not expired.
func (b *Driver[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	if err := b.check(ctx); err != nil {
		var zero V
		return zero, false, err
	}
	value, found := b.cache.GetIfPresent(key)
	return value, found, nil
}

// Set stores the value under the key, evicting other entries when the capacity is exceeded.
func (b *Driver[K, V]) Set(ctx context.Context, key K, value V) error {
	if err := b.check(ctx); err != nil {
		return err
	}
	b.cache.Set(key, value)
	return nil
}

// Has reports whether the key is present without returning the value.
func (b *Driver[K, V]) Has(ctx context.Context, key K) (bool, error) {
	if err := b.check(ctx); err != nil {
		return false, err
	}
	_, found := b.cache.GetIfPresent(key)
	return found, nil
}

// Delete removes the key. Deleting a missing key is not an error.
func (b *Driver[K, V]) Delete(ctx context.Context, key K) error {
	if err := b.check(ctx); err != nil {
		return err
	}
	b.cache.Invalidate(key)
	return nil
}

// Clear removes every entry.
func (b *Driver[K, V]) Clear(ctx context.Context) error {
	if err := b.check(ctx); err != nil {
		return err
	}
	b.cache.InvalidateAll()
	return nil
}

// Len cleans up expired entries and returns the estimated number of remaining entries.
func (b *Driver[K, V]) Len(ctx context.Context) (int, error) {
	if err := b.check(ctx); err != nil {
		return 0, err
	}
	b.cache.CleanUp()
	return b.cache.EstimatedSize(), nil
}

// Close invalidates all entries, stops the internal maintenance goroutines, and makes subsequent
// operations return cache.ErrClosed. Close is idempotent.
func (b *Driver[K, V]) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		b.cache.InvalidateAll()
		b.cache.StopAllGoroutines()
	})
	return nil
}

var _ cache.Driver[int, int] = (*Driver[int, int])(nil)
