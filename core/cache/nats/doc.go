// Package nats provides distributed cache.Driver and cache.VersionedDriver implementations backed
// by NATS JetStream key-value buckets.
//
// # Architecture
//
// Each driver owns exactly one JetStream KV bucket and shares a core/nats.Client connection with the
// rest of the transport. Keys are converted to bucket keys with a cache.KeyEncoder and values are
// serialized with a cache.Codec, so the stored form is fully controlled by the caller. Delete uses a
// purge so per-key history does not accumulate in the underlying stream.
//
// Entry expiry is a bucket-wide property: the TTL configured in jetstream.KeyValueConfig is applied by
// the server to every entry, and clients cannot override it per key. Because of that, BindExisting
// validates that the existing bucket's TTL, history, replica count, and storage match the configured
// values and refuses to bind otherwise.
// Use Provision mode Ensure to create or update the bucket (and its TTL) from the application.
//
// The bucket content is shared by every process using it, which makes the driver a building block for
// cross-replica caches. Drivers do not watch for updates: reads hit the server, so changes made by
// another process are visible on the next operation.
//
// The versioned facade exposes JetStream revisions for optimistic concurrency. Create is
// create-if-absent, Update and DeleteRevision require the current revision, and Keys decodes bucket
// keys through a cache.KeyDecoder. Revision conflicts match cache.ErrConflict.
//
// # Usage
//
//	client, err := corenats.Connect(ctx, corenats.Config{URLs: []string{"nats://localhost:4222"}}, log)
//	if err != nil {
//	    return err
//	}
//	driver, err := nats.New[int, Account](
//	    ctx, client,
//	    cache.JSONKeyEncoder[int]{},
//	    cache.JSONCodec[Account]{},
//	    nats.Config{
//	        Bucket:    jetstream.KeyValueConfig{Bucket: "accounts", TTL: time.Hour},
//	        Provision: nats.Ensure,
//	    },
//	)
//
// Closing the driver only marks it closed: neither the shared client nor the bucket is touched.
package nats
