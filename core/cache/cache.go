package cache

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrClosed is returned by every cache operation after Close.
	ErrClosed = errors.New("cache is closed")
	// ErrConflict is returned when a create-only write finds an existing key or a conditional
	// update/delete observes a different revision.
	ErrConflict = errors.New("cache entry revision conflict")
	// ErrKeyDecodingUnsupported is returned by Keys when a driver was configured with an encoder
	// that cannot decode persisted keys back to their typed form.
	ErrKeyDecodingUnsupported = errors.New("cache key decoding is not supported")
)

// Driver is the storage contract behind Cache. Implementations live in the memory and nats
// subpackages. Get reports a miss with a false second result instead of an error; Len counts the
// entries currently stored.
type Driver[K comparable, V any] interface {
	Get(context.Context, K) (V, bool, error)
	Set(context.Context, K, V) error
	Has(context.Context, K) (bool, error)
	Delete(context.Context, K) error
	Clear(context.Context) error
	Len(context.Context) (int, error)
	Close(context.Context) error
}

// Entry is a versioned cache value. Revision is driver-defined and can be passed to Update or
// DeleteRevision for optimistic concurrency control. CreatedAt is the time of this revision.
type Entry[V any] struct {
	Value     V
	Revision  uint64
	CreatedAt time.Time
}

// VersionedDriver extends Driver with atomic operations suitable for shared state. Implementations
// map conflicting creates and stale revisions to ErrConflict.
type VersionedDriver[K comparable, V any] interface {
	Driver[K, V]
	GetEntry(context.Context, K) (Entry[V], bool, error)
	Create(context.Context, K, V) (uint64, error)
	Update(context.Context, K, V, uint64) (uint64, error)
	DeleteRevision(context.Context, K, uint64) error
	Keys(context.Context) ([]K, error)
}

// Cache is a typed facade over a Driver. Construct it with New and share it freely: the cache adds no
// state of its own and is safe for concurrent use as long as the driver is.
type Cache[K comparable, V any] struct {
	driver Driver[K, V]
}

// New wraps a driver into the user-facing cache facade.
func New[K comparable, V any](driver Driver[K, V]) *Cache[K, V] {
	return &Cache[K, V]{driver: driver}
}

// VersionedCache is a typed facade over a VersionedDriver. It embeds the ordinary cache facade and
// adds optimistic-concurrency operations without expanding the basic Driver contract.
type VersionedCache[K comparable, V any] struct {
	*Cache[K, V]
	driver VersionedDriver[K, V]
}

// NewVersioned wraps a versioned driver into a user-facing facade.
func NewVersioned[K comparable, V any](driver VersionedDriver[K, V]) *VersionedCache[K, V] {
	return &VersionedCache[K, V]{Cache: New[K, V](driver), driver: driver}
}

// GetEntry returns a value together with its revision metadata.
func (c *VersionedCache[K, V]) GetEntry(ctx context.Context, key K) (Entry[V], bool, error) {
	return c.driver.GetEntry(ctx, key)
}

// Create stores a value only when the key does not currently exist.
func (c *VersionedCache[K, V]) Create(ctx context.Context, key K, value V) (uint64, error) {
	return c.driver.Create(ctx, key, value)
}

// Update replaces a value only when revision is still current.
func (c *VersionedCache[K, V]) Update(ctx context.Context, key K, value V, revision uint64) (uint64, error) {
	return c.driver.Update(ctx, key, value, revision)
}

// DeleteRevision removes a value only when revision is still current.
func (c *VersionedCache[K, V]) DeleteRevision(ctx context.Context, key K, revision uint64) error {
	return c.driver.DeleteRevision(ctx, key, revision)
}

// Keys returns all currently present typed keys.
func (c *VersionedCache[K, V]) Keys(ctx context.Context) ([]K, error) {
	return c.driver.Keys(ctx)
}

// Get returns the cached value for the key. A missing key yields a zero value, false, and a nil error.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	return c.driver.Get(ctx, key)
}

// Set stores the value under the key, replacing any previous entry.
func (c *Cache[K, V]) Set(ctx context.Context, key K, value V) error {
	return c.driver.Set(ctx, key, value)
}

// Has reports whether the key is present without decoding the value.
func (c *Cache[K, V]) Has(ctx context.Context, key K) (bool, error) {
	return c.driver.Has(ctx, key)
}

// Delete removes the key. Deleting a missing key is not an error.
func (c *Cache[K, V]) Delete(ctx context.Context, key K) error {
	return c.driver.Delete(ctx, key)
}

// Clear removes every entry from the driver.
func (c *Cache[K, V]) Clear(ctx context.Context) error {
	return c.driver.Clear(ctx)
}

// Len returns the number of entries currently stored.
func (c *Cache[K, V]) Len(ctx context.Context) (int, error) {
	return c.driver.Len(ctx)
}

// Close releases driver resources. Subsequent operations return ErrClosed.
func (c *Cache[K, V]) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}
