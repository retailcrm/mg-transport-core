package logger

import (
	"path"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	LoggerContextKey     = "logger"
	LoggerRealContextKey = "loggerReal"
)

// GinMiddleware will construct Gin middleware which will log requests and provide logger with unique request ID.
func GinMiddleware(log Logger, skipPaths ...string) gin.HandlerFunc {
	var (
		skip      map[string]struct{}
		matchSkip []*skippedPath
	)
	if length := len(skipPaths); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, path := range skipPaths {
			if skipped, ok := newSkippedPath(path); ok {
				matchSkip = append(matchSkip, skipped)
				continue
			}
			skip[path] = struct{}{}
		}
	}

	nilLogger := NewNil()

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		_, shouldSkip := skip[path]
		streamID := generateStreamID()
		log := log.With(StreamID(streamID))

		if !shouldSkip && len(matchSkip) > 0 {
			for _, skipper := range matchSkip {
				if skipper.match(path) {
					shouldSkip = true
					break
				}
			}
		}

		if shouldSkip {
			c.Set(LoggerRealContextKey, log)
			log = nilLogger
		}

		c.Set(StreamIDAttr, streamID)
		c.Set(LoggerContextKey, log)

		// Process request
		c.Next()

		end := time.Now()
		if raw != "" {
			path = path + "?" + raw
		}

		if !shouldSkip {
			log.Info("request",
				zap.String(HandlerAttr, "GIN"),
				zap.String("startTime", start.Format(time.RFC3339)),
				zap.String("endTime", end.Format(time.RFC3339)),
				zap.Any("latency", end.Sub(start)/time.Millisecond),
				zap.String("remoteAddress", c.ClientIP()),
				zap.String(HTTPMethodAttr, c.Request.Method),
				zap.String("path", path),
				zap.Int("bodySize", c.Writer.Size()),
			)
		}
	}
}

func MustGet(c *gin.Context) Logger {
	return c.MustGet("logger").(Logger)
}

func MustGetReal(c *gin.Context) Logger {
	log, ok := c.Get(LoggerContextKey)
	if _, isNil := log.(*Nil); !ok || isNil {
		return c.MustGet(LoggerRealContextKey).(Logger)
	}
	return log.(Logger)
}

var (
	hasParamsMatcher         = regexp.MustCompile(`/:\w+`)
	hasWildcardParamsMatcher = regexp.MustCompile(`/\*\w+.*`)
)

type skippedPath struct {
	path string
	expr *regexp.Regexp
}

// newSkippedPath returns new path skipping struct. It returns nil, false if expr is simple and
// no complex logic is needed.
func newSkippedPath(expr string) (result *skippedPath, compatible bool) {
	hasParams, hasWildcard := hasParamsMatcher.MatchString(expr), hasWildcardParamsMatcher.MatchString(expr)
	if !hasParams && !hasWildcard {
		return nil, false
	}
	if hasWildcard {
		return &skippedPath{expr: matcherForWildcard(expr)}, true
	}
	return &skippedPath{path: matcherForPath(expr)}, true
}

func matcherForWildcard(expr string) *regexp.Regexp {
	return regexp.MustCompile(hasWildcardParamsMatcher.ReplaceAllString(expr, "/[\\w/]+"))
}

func matcherForPath(expr string) string {
	return hasParamsMatcher.ReplaceAllString(expr, "/*")
}

func (p *skippedPath) match(route string) bool {
	if p.expr != nil {
		return p.expr.MatchString(route)
	}
	result, err := path.Match(p.path, route)
	return result && err == nil
}
