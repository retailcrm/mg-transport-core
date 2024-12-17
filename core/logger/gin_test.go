package logger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGinMiddleware(t *testing.T) {
	log := newBufferLoggerSilent()
	rr := httptest.NewRecorder()
	r := gin.New()
	r.Use(GinMiddleware(log))
	r.GET("/mine", func(c *gin.Context) {
		log := MustGet(c)
		log.Info("some very important message")
		c.JSON(http.StatusOK, gin.H{})
	})
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/mine", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	items, err := newJSONBufferedLogger(log).ScanAll()
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.NotEmpty(t, items[0].StreamID)
	require.NotEmpty(t, items[1].StreamID)
	assert.Equal(t, "some very important message", items[0].Message)
	require.NotEmpty(t, items[1].Context)
	assert.NotEmpty(t, items[1].Context["startTime"])
	assert.NotEmpty(t, items[1].Context["endTime"])
	assert.True(t, func() bool {
		_, ok := items[1].Context["latency"]
		return ok
	}())
	assert.NotEmpty(t, items[1].Context["remoteAddress"])
	assert.NotEmpty(t, items[1].Context[HTTPMethodAttr])
	assert.NotEmpty(t, items[1].Context["path"])
	assert.NotEmpty(t, items[1].Context["bodySize"])
}

func TestGinMiddleware_SkipPaths(t *testing.T) {
	log := newBufferLoggerSilent()
	rr := httptest.NewRecorder()
	r := gin.New()
	r.Use(GinMiddleware(log, "/hidden", "/hidden/:id", "/superhidden/*id"))
	r.GET("/hidden", func(c *gin.Context) {
		log := MustGet(c)
		log.Info("hidden message from /hidden")
		realLog := MustGetReal(c)
		realLog.Info("visible message from /hidden")
		c.JSON(http.StatusOK, gin.H{})
	})
	r.GET("/logged", func(c *gin.Context) {
		log := MustGet(c)
		log.Info("visible message from /logged")
		c.JSON(http.StatusOK, gin.H{})
	})
	r.GET("/hidden/:id", func(c *gin.Context) {
		log := MustGet(c)
		log.Info("hidden message from /hidden/:id")
		c.JSON(http.StatusOK, gin.H{})
	})
	r.GET("/superhidden/*id", func(c *gin.Context) {
		log := MustGet(c)
		log.Info("hidden message from /superhidden/*id")
		c.JSON(http.StatusOK, gin.H{})
	})

	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/hidden", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/logged", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/hidden/param", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/superhidden/param/1/2/3", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	items, err := newJSONBufferedLogger(log).ScanAll()
	require.NoError(t, err)
	require.Len(t, items, 3, printEntries(items))
	assert.Equal(t, "visible message from /hidden", items[0].Message, printEntries(items))
	assert.Equal(t, "visible message from /logged", items[1].Message, printEntries(items))
}

func TestSkippedPath(t *testing.T) {
	cases := map[string]map[string]bool{
		"/hidden/:id": {
			"/hidden/1":   true,
			"/hidden/2/3": false,
		},
		"/hidden/*id": {
			"/hidden/1":   true,
			"/hidden/2/3": true,
		},
	}

	for pattern, items := range cases {
		matcher, ok := newSkippedPath(pattern)
		require.True(t, ok)

		for item, result := range items {
			assert.Equal(t, result, matcher.match(item), `"%s" does not match "%s", internals: %#v`,
				pattern, item, matcher)
		}
	}
}

func printEntries(entries []logRecord) string {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}
