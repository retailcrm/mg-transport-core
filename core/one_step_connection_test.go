package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	retailcrm "github.com/retailcrm/api-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustGetConnectRequest_Success(t *testing.T) {
	assert.Equal(t, retailcrm.ConnectRequest{}, MustGetConnectRequest(&gin.Context{
		Keys: map[string]interface{}{
			"request": retailcrm.ConnectRequest{},
		},
	}))
}

func TestMustGetConnectRequest_Failure(t *testing.T) {
	assert.Panics(t, func() {
		MustGetConnectRequest(&gin.Context{
			Keys: map[string]interface{}{},
		})
	})
	assert.Panics(t, func() {
		MustGetConnectRequest(&gin.Context{})
	})
	assert.Panics(t, func() {
		MustGetConnectRequest(nil)
	})
}

func TestConnectionConfig(t *testing.T) {
	scopes := []string{
		"integration_read",
		"integration_write",
	}
	registerURL := "https://example.com"
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.GET("/", ConnectionConfig(registerURL, scopes))

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	g.ServeHTTP(rr, req)

	var cc retailcrm.ConnectionConfigResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&cc))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, retailcrm.ConnectionConfigResponse{
		SuccessfulResponse: retailcrm.SuccessfulResponse{
			Success: true,
		},
		Scopes:      scopes,
		RegisterURL: registerURL,
	}, cc)
}

func TestVerifyConnectRequest_NoData(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.POST("/", VerifyConnectRequest("secret"))

	req, err := http.NewRequest(http.MethodPost, "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	g.ServeHTTP(rr, req)

	var resp retailcrm.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, retailcrm.ErrorResponse{ErrorMessage: "No data provided"}, resp)
}

func TestVerifyConnectRequest_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.POST("/", VerifyConnectRequest("secret"))

	req, err := http.NewRequest(
		http.MethodPost, "/", strings.NewReader(url.Values{"register": {"invalid json"}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	g.ServeHTTP(rr, req)

	var resp retailcrm.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, retailcrm.ErrorResponse{ErrorMessage: "Invalid JSON provided"}, resp)
}

func TestVerifyConnectRequest_InvalidToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.POST("/", VerifyConnectRequest("secret"))

	data, err := json.Marshal(retailcrm.ConnectRequest{
		Token:  "token",
		APIKey: "key",
		URL:    "https://example.com",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(
		http.MethodPost, "/", strings.NewReader(url.Values{"register": {string(data)}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	g.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "{}", rr.Body.String())
}

func TestVerifyConnectRequest_OK(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.POST("/", VerifyConnectRequest("secret"), func(c *gin.Context) {
		_ = MustGetConnectRequest(c)
		c.AbortWithStatus(http.StatusCreated)
	})

	data, err := json.Marshal(retailcrm.ConnectRequest{
		Token:  createConnectToken("key", "secret"),
		APIKey: "key",
		URL:    "https://example.com",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(
		http.MethodPost, "/", strings.NewReader(url.Values{"register": {string(data)}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	g.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func createConnectToken(apiKey, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(apiKey)); err != nil {
		panic(err)
	}
	return hex.EncodeToString(mac.Sum(nil))
}
