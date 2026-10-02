// Package memory provides a process-local cache.Driver implementation.
//
// # Architecture
//
// The driver stores entries in an otter cache (a high-performance, S3-FIFO based cache library) with
// a hard capacity and an optional write-time TTL: entries expire a fixed duration after they were
// written. Setting a value when the cache is full evicts the least valuable entries according to the
// otter admission policy. All operations are lock-free and safe for concurrent use; Len triggers an
// internal cleanup first so expired entries are not counted.
//
// Because entries live only in the process, the driver suits per-instance caches (connection
// objects, resolved tokens, API responses) where a miss can be recomputed. For caches shared across
// transport replicas use the nats driver.
//
// # Usage
//
//	driver, err := memory.New[int, Account](memory.Options{
//	    Capacity: 1_000,
//	    TTL:      time.Hour,
//	})
//	if err != nil {
//	    return err
//	}
//	accounts := cache.New(driver)
package memory
