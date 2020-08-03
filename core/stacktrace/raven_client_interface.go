package stacktrace

import "github.com/getsentry/raven-go"

// RavenClientInterface includes all necessary calls from *raven.Client. Therefore, it can be mocked or replaced.
type RavenClientInterface interface {
	CaptureMessageAndWait(message string, tags map[string]string, interfaces ...raven.Interface) string
	CaptureErrorAndWait(err error, tags map[string]string, interfaces ...raven.Interface) string
	IncludePaths() []string
}
