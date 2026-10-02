package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyCodecsRoundTrip(t *testing.T) {
	stringCodec := StringKeyEncoder{}
	encoded, err := stringCodec.EncodeKey("account.42")
	require.NoError(t, err)
	decoded, err := stringCodec.DecodeKey(encoded)
	require.NoError(t, err)
	assert.Equal(t, "account.42", decoded)

	type key struct {
		AccountID int
		Kind      string
	}
	jsonCodec := JSONKeyEncoder[key]{}
	want := key{AccountID: 42, Kind: "outbound"}
	encoded, err = jsonCodec.EncodeKey(want)
	require.NoError(t, err)
	got, err := jsonCodec.DecodeKey(encoded)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = jsonCodec.DecodeKey("not base64!")
	require.Error(t, err)
}
