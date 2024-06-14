package logger

import (
	"net/http"
	"testing"

	"github.com/h2non/gock"
	retailcrm "github.com/retailcrm/api-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIClientAdapter(t *testing.T) {
	log := newJSONBufferedLogger(nil)
	client := retailcrm.New("https://example.com", "test_key").WithLogger(APIClientAdapter(log.Logger()))
	client.Debug = true

	defer gock.Off()
	gock.New("https://example.com").
		Get("/api/credentials").
		Reply(http.StatusOK).
		JSON(retailcrm.CredentialResponse{Success: true})

	_, _, err := client.APICredentials()
	require.NoError(t, err)

	entries, err := log.ScanAll()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "DEBUG", entries[0].LevelName)
	assert.True(t, entries[0].DateTime.Valid)
	assert.Equal(t, "API Request", entries[0].Message)
	assert.Equal(t, "test_key", entries[0].Context["key"])
	assert.Equal(t, "https://example.com/api/credentials", entries[0].Context["url"])

	assert.Equal(t, "DEBUG", entries[1].LevelName)
	assert.True(t, entries[1].DateTime.Valid)
	assert.Equal(t, "API Response", entries[1].Message)
	assert.Equal(t, map[string]interface{}{"success": true}, entries[1].Context["body"])
}
