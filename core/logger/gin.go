package logger

import (
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"
)

// GinMiddleware will construct Gin middleware which will log requests.
func GinMiddleware(log Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		end := time.Now()
		if raw != "" {
			path = path + "?" + raw
		}

		log.Info("[GIN] request",
			slog.String("startTime", start.Format(time.RFC3339)),
			slog.String("endTime", end.Format(time.RFC3339)),
			slog.Any("latency", end.Sub(start)/time.Millisecond),
			slog.String("remoteAddress", c.ClientIP()),
			slog.String(HTTPMethodAttr, c.Request.Method),
			slog.String("path", path),
			slog.Int("bodySize", c.Writer.Size()),
		)
	}
}
