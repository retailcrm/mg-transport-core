package nats_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/retailcrm/mg-transport-core/v2/core/cache"
	cachenats "github.com/retailcrm/mg-transport-core/v2/core/cache/nats"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	corenats "github.com/retailcrm/mg-transport-core/v2/core/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startNATS(t *testing.T) *corenats.Client {
	t.Helper()
	srv, err := server.NewServer(&server.Options{
		JetStream: true,
		StoreDir:  t.TempDir(),
		Port:      -1,
	})
	require.NoError(t, err)
	srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)

	client, err := corenats.Connect(
		t.Context(),
		corenats.Config{URLs: []string{srv.ClientURL()}},
		logger.NewNil(),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func newDriver(
	t *testing.T,
	client *corenats.Client,
	bucket string,
	provision cachenats.ProvisionMode,
	ttl time.Duration,
) *cachenats.Driver[int, string] {
	t.Helper()
	driver, err := cachenats.New(
		t.Context(),
		client,
		cache.JSONKeyEncoder[int]{},
		cache.JSONCodec[string]{},
		cachenats.Config{
			Bucket: jetstream.KeyValueConfig{
				Bucket:  bucket,
				TTL:     ttl,
				Storage: jetstream.MemoryStorage,
			},
			Provision: provision,
		},
	)
	require.NoError(t, err)
	return driver
}

func TestDriverLifecycleAndVisibility(t *testing.T) {
	client := startNATS(t)
	first := newDriver(t, client, "CACHE_LIFECYCLE", cachenats.Ensure, 0)
	second := newDriver(t, client, "CACHE_LIFECYCLE", cachenats.BindExisting, 0)

	value, found, err := first.Get(t.Context(), 404)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, value)

	require.NoError(t, first.Set(t.Context(), 1, "one"))
	require.NoError(t, first.Set(t.Context(), 2, "two"))
	require.NoError(t, first.Set(t.Context(), 1, "updated"))
	value, found, err = second.Get(t.Context(), 1)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "updated", value)

	found, err = second.Has(t.Context(), 2)
	require.NoError(t, err)
	assert.True(t, found)
	length, err := second.Len(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, length)

	require.NoError(t, second.Delete(t.Context(), 2))
	require.NoError(t, second.Delete(t.Context(), 404))
	require.NoError(t, second.Clear(t.Context()))
	require.NoError(t, second.Clear(t.Context()))
	length, err = first.Len(t.Context())
	require.NoError(t, err)
	assert.Zero(t, length)

	require.NoError(t, first.Close(t.Context()))
	require.NoError(t, first.Close(t.Context()))
	require.ErrorIs(t, first.Set(t.Context(), 3, "closed"), cache.ErrClosed)

	// Closing a driver must not close its shared client or remove its bucket.
	require.NoError(t, second.Set(t.Context(), 3, "still open"))
	value, found, err = second.Get(t.Context(), 3)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "still open", value)
}

func TestDriverExpiresAfterLastSet(t *testing.T) {
	client := startNATS(t)
	driver := newDriver(t, client, "CACHE_TTL", cachenats.Ensure, 200*time.Millisecond)

	require.NoError(t, driver.Set(t.Context(), 1, "first"))
	time.Sleep(125 * time.Millisecond)
	require.NoError(t, driver.Set(t.Context(), 1, "second"))
	time.Sleep(125 * time.Millisecond)

	value, found, err := driver.Get(t.Context(), 1)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "second", value)

	require.Eventually(t, func() bool {
		_, found, getErr := driver.Get(t.Context(), 1)
		return getErr == nil && !found
	}, 2*time.Second, 20*time.Millisecond)
}

func TestBindValidatesTTL(t *testing.T) {
	client := startNATS(t)
	_ = newDriver(t, client, "CACHE_BIND", cachenats.Ensure, time.Second)

	_, err := cachenats.New(
		t.Context(),
		client,
		cache.StringKeyEncoder{},
		cache.JSONCodec[string]{},
		cachenats.Config{Bucket: jetstream.KeyValueConfig{Bucket: "CACHE_BIND"}},
	)
	require.ErrorContains(t, err, "TTL")
}

func TestDriverValidation(t *testing.T) {
	client := startNATS(t)
	config := cachenats.Config{Bucket: jetstream.KeyValueConfig{Bucket: "VALIDATION"}, Provision: cachenats.Ensure}

	_, err := cachenats.New[string, string](t.Context(), nil, cache.StringKeyEncoder{}, cache.JSONCodec[string]{}, config)
	require.Error(t, err)
	_, err = cachenats.New[string, string](t.Context(), client, nil, cache.JSONCodec[string]{}, config)
	require.Error(t, err)
	_, err = cachenats.New[string, string](t.Context(), client, cache.StringKeyEncoder{}, nil, config)
	require.Error(t, err)
	_, err = cachenats.New[string, string](
		t.Context(),
		client,
		cache.StringKeyEncoder{},
		cache.JSONCodec[string]{},
		cachenats.Config{Provision: cachenats.Ensure},
	)
	require.Error(t, err)
}

type failingKeyEncoder struct{}

func (failingKeyEncoder) EncodeKey(string) (string, error) {
	return "", errors.New("key failure")
}

type failingCodec struct{}

func (failingCodec) Encode(string) ([]byte, error) {
	return nil, errors.New("value failure")
}

func (failingCodec) Decode([]byte) (string, error) {
	return "", errors.New("value failure")
}

func TestDriverReportsCodecErrors(t *testing.T) {
	client := startNATS(t)
	config := cachenats.Config{
		Bucket:    jetstream.KeyValueConfig{Bucket: "CACHE_CODEC", Storage: jetstream.MemoryStorage},
		Provision: cachenats.Ensure,
	}
	badKey, err := cachenats.New(t.Context(), client, failingKeyEncoder{}, cache.JSONCodec[string]{}, config)
	require.NoError(t, err)
	require.ErrorContains(t, badKey.Set(t.Context(), "key", "value"), "encode NATS cache key")

	badValue, err := cachenats.New(t.Context(), client, cache.StringKeyEncoder{}, failingCodec{}, config)
	require.NoError(t, err)
	require.ErrorContains(t, badValue.Set(t.Context(), "key", "value"), "encode NATS cache value")
}

func TestDriverHonorsContext(t *testing.T) {
	client := startNATS(t)
	driver := newDriver(t, client, "CACHE_CONTEXT", cachenats.Ensure, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, driver.Set(ctx, 1, "value"), context.Canceled)
}

func TestVersionedDriverLifecycle(t *testing.T) {
	client := startNATS(t)
	driver := newDriver(t, client, "CACHE_VERSIONED", cachenats.Ensure, 0)
	versioned := cache.NewVersioned[int, string](driver)

	entry, found, err := versioned.GetEntry(t.Context(), 1)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, entry)

	revision, err := versioned.Create(t.Context(), 1, "first")
	require.NoError(t, err)
	assert.NotZero(t, revision)
	_, err = versioned.Create(t.Context(), 1, "duplicate")
	require.ErrorIs(t, err, cache.ErrConflict)

	entry, found, err = versioned.GetEntry(t.Context(), 1)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "first", entry.Value)
	assert.Equal(t, revision, entry.Revision)
	assert.False(t, entry.CreatedAt.IsZero())

	nextRevision, err := versioned.Update(t.Context(), 1, "second", revision)
	require.NoError(t, err)
	assert.Greater(t, nextRevision, revision)
	_, err = versioned.Update(t.Context(), 1, "stale", revision)
	require.ErrorIs(t, err, cache.ErrConflict)
	require.ErrorIs(t, versioned.DeleteRevision(t.Context(), 1, revision), cache.ErrConflict)
	require.NoError(t, versioned.DeleteRevision(t.Context(), 1, nextRevision))

	_, found, err = versioned.GetEntry(t.Context(), 1)
	require.NoError(t, err)
	assert.False(t, found)
	_, err = versioned.Create(t.Context(), 1, "recreated")
	require.NoError(t, err)
}

func TestVersionedDriverKeys(t *testing.T) {
	client := startNATS(t)
	driver := newDriver(t, client, "CACHE_KEYS", cachenats.Ensure, 0)
	versioned := cache.NewVersioned[int, string](driver)

	keys, err := versioned.Keys(t.Context())
	require.NoError(t, err)
	assert.Empty(t, keys)
	_, err = versioned.Create(t.Context(), 2, "two")
	require.NoError(t, err)
	_, err = versioned.Create(t.Context(), 1, "one")
	require.NoError(t, err)
	keys, err = versioned.Keys(t.Context())
	require.NoError(t, err)
	slices.Sort(keys)
	assert.Equal(t, []int{1, 2}, keys)
}

func TestVersionedDriverRequiresKeyDecoderForKeys(t *testing.T) {
	client := startNATS(t)
	config := cachenats.Config{
		Bucket:    jetstream.KeyValueConfig{Bucket: "CACHE_KEYS_ENCODER", Storage: jetstream.MemoryStorage},
		Provision: cachenats.Ensure,
	}
	driver, err := cachenats.New(t.Context(), client, failingKeyEncoder{}, cache.JSONCodec[string]{}, config)
	require.NoError(t, err)
	_, err = driver.Keys(t.Context())
	require.ErrorIs(t, err, cache.ErrKeyDecodingUnsupported)
}
