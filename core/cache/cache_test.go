package cache_test

import (
	"context"
	"testing"

	"github.com/retailcrm/mg-transport-core/v2/core/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type driverStub struct {
	items map[string]int
}

func (b *driverStub) Get(_ context.Context, key string) (int, bool, error) {
	value, found := b.items[key]
	return value, found, nil
}

func (b *driverStub) Set(_ context.Context, key string, value int) error {
	b.items[key] = value
	return nil
}

func (b *driverStub) Has(_ context.Context, key string) (bool, error) {
	_, found := b.items[key]
	return found, nil
}

func (b *driverStub) Delete(_ context.Context, key string) error {
	delete(b.items, key)
	return nil
}

func (b *driverStub) Clear(context.Context) error {
	clear(b.items)
	return nil
}

func (b *driverStub) Len(context.Context) (int, error) {
	return len(b.items), nil
}

func (b *driverStub) Close(context.Context) error {
	return nil
}

func TestCacheDelegatesToDriver(t *testing.T) {
	c := cache.New[string, int](&driverStub{items: make(map[string]int)})

	require.NoError(t, c.Set(t.Context(), "answer", 42))
	value, found, err := c.Get(t.Context(), "answer")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 42, value)

	found, err = c.Has(t.Context(), "answer")
	require.NoError(t, err)
	assert.True(t, found)

	length, err := c.Len(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, length)

	require.NoError(t, c.Delete(t.Context(), "answer"))
	require.NoError(t, c.Clear(t.Context()))
	require.NoError(t, c.Close(t.Context()))
}

func TestCodecs(t *testing.T) {
	t.Run("JSON value", func(t *testing.T) {
		codec := cache.JSONCodec[map[string]int]{}
		encoded, err := codec.Encode(map[string]int{"answer": 42})
		require.NoError(t, err)
		decoded, err := codec.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"answer": 42}, decoded)
	})

	t.Run("bytes are copied", func(t *testing.T) {
		codec := cache.BytesCodec{}
		original := []byte("value")
		encoded, err := codec.Encode(original)
		require.NoError(t, err)
		encoded[0] = 'V'
		assert.Equal(t, []byte("value"), original)

		decoded, err := codec.Decode(encoded)
		require.NoError(t, err)
		decoded[0] = 'x'
		assert.Equal(t, []byte("Value"), encoded)
	})

	t.Run("typed key", func(t *testing.T) {
		encoded, err := (cache.JSONKeyEncoder[int]{}).EncodeKey(42)
		require.NoError(t, err)
		assert.Equal(t, "NDI", encoded)
	})
}
