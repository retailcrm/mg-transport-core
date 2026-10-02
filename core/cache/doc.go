// Package cache provides typed cache and versioned key-value facades.
//
// # Architecture
//
// Driver is the basic storage interface (get, set, has, delete, clear, length, close) implemented by
// the memory and nats subpackages. Cache is the user-facing facade that
// simply forwards to a driver, giving transports a stable, domain-typed API:
//
//	                 ┌──────────────┐
//	transport code ─►│ Cache[K, V]  │ get / set / has / delete / clear / len
//	                 └──────┬───────┘
//	                        ▼
//	                 Driver[K, V] (interface)
//	                  ┌─────┴─────┐
//	          memory  │           │  nats (JetStream KV)
//	        otter, TTL-bounded  bucket, shared across processes
//
// VersionedDriver and VersionedCache add create-only writes, revision-checked updates and deletes,
// revision metadata, and typed key listing without expanding the basic Driver contract. The NATS
// driver implements this contract for shared state and coordination use cases.
//
// Persistent drivers exchange values with storage as bytes, so they also need a Codec for values and
// a KeyEncoder for keys (both defined in this package). JSONCodec, BytesCodec, StringKeyEncoder, and
// JSONKeyEncoder cover the common cases and decode keys for typed listing; transports with
// domain-specific encodings can plug in their
// own implementations. Drivers with fixed server-side TTLs (such as JetStream KV buckets) validate
// the configuration at construction time.
//
// # Usage
//
//	driver, err := memory.New[int, Account](memory.Options{Capacity: 1_000, TTL: time.Hour})
//	if err != nil {
//	    return err
//	}
//	accounts := cache.New(driver)
//
//	if err := accounts.Set(ctx, account.ID, account); err != nil {
//	    return err
//	}
//	account, found, err := accounts.Get(ctx, accountID)
//
// All methods accept a context so network drivers can honor deadlines and cancellation. After Close,
// every method returns ErrClosed.
package cache
