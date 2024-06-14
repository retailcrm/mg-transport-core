package logger

import (
	"net/http"
	"testing"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/retailcrm/mg-transport-api-client-go/v1"
)

func TestMGTransportClientAdapter(t *testing.T) {
	httpClient := &http.Client{}
	log := newJSONBufferedLogger(nil)
	client := v1.NewWithClient("https://mg.dev", "test_token", httpClient).
		WithLogger(MGTransportClientAdapter(log.Logger()))
	client.Debug = true

	defer gock.Off()
	gock.New("https://mg.dev").
		Get("/api/transport/v1/channels").
		Reply(http.StatusOK).
		JSON([]v1.ChannelListItem{{ID: 123}})

	_, _, err := client.TransportChannels(v1.Channels{})
	require.NoError(t, err)

	entries, err := log.ScanAll()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "DEBUG", entries[0].LevelName)
	assert.True(t, entries[0].DateTime.Valid)
	assert.Equal(t, "MG TRANSPORT API Request", entries[0].Message)
	assert.Equal(t, http.MethodGet, entries[0].Context["method"])
	assert.Equal(t, "test_token", entries[0].Context["token"])
	assert.Equal(t, "https://mg.dev/api/transport/v1/channels?", entries[0].Context["url"])

	assert.Equal(t, "DEBUG", entries[1].LevelName)
	assert.True(t, entries[1].DateTime.Valid)
	assert.Equal(t, "MG TRANSPORT API Response", entries[1].Message)
	assert.Equal(t, float64(123), entries[1].Context["body"].([]interface{})[0].(map[string]interface{})["id"])
}
