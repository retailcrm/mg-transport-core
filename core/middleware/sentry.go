package middleware

import (
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

var ginContextSentryKey = "middleware-sentry"

type Sentry interface {
	CaptureException(c *gin.Context, exception error)
	CaptureMessage(c *gin.Context, message string)
	CaptureEvent(c *gin.Context, event *sentry.Event)
}

func InjectSentry(sentry Sentry) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ginContextSentryKey, sentry)
	}
}

func GetSentry(c *gin.Context) (Sentry, bool) {
	sentryValue, ok := c.Get(ginContextSentryKey)
	if !ok {
		return nil, false
	}
	obj, ok := sentryValue.(Sentry)
	if !ok || obj == nil {
		return nil, false
	}
	return obj, true
}

func MustGetSentry(c *gin.Context) Sentry {
	if obj, ok := GetSentry(c); ok && obj != nil {
		return obj
	}
	panic("obj not found in context")
}

func CaptureException(c *gin.Context, exception error) {
	obj, found := GetSentry(c)
	if !found {
		return
	}
	obj.CaptureException(c, exception)
}

func CaptureEvent(c *gin.Context, event *sentry.Event) {
	obj, found := GetSentry(c)
	if !found {
		return
	}
	obj.CaptureEvent(c, event)
}

func CaptureMessage(c *gin.Context, message string) {
	obj, found := GetSentry(c)
	if !found {
		return
	}
	obj.CaptureMessage(c, message)
}
