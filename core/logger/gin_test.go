package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGinMiddleware(t *testing.T) {
	log := newBufferLogger()
	rr := httptest.NewRecorder()
	r := gin.New()
	r.Use(GinMiddleware(log))
	r.GET("/mine", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/mine", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	items, err := newJSONBufferedLogger(log).ScanAll()
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotEmpty(t, items[0].Context)
	assert.NotEmpty(t, items[0].Context["startTime"])
	assert.NotEmpty(t, items[0].Context["endTime"])
	assert.True(t, func() bool {
		_, ok := items[0].Context["latency"]
		return ok
	}())
	assert.NotEmpty(t, items[0].Context["remoteAddress"])
	assert.NotEmpty(t, items[0].Context[HTTPMethodAttr])
	assert.NotEmpty(t, items[0].Context["path"])
	assert.NotEmpty(t, items[0].Context["bodySize"])
}
