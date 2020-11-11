package stacktrace

import "github.com/getsentry/raven-go"

// RavenClientInterface includes all necessary calls from *raven.Client. Therefore, it can be mocked or replaced.
type RavenClientInterface interface {
	SetIgnoreErrors(errs []string) error
	SetDSN(dsn string) error
	SetRelease(release string)
	SetEnvironment(environment string)
	SetDefaultLoggerName(name string)
	SetSampleRate(rate float32) error
	Capture(packet *raven.Packet, captureTags map[string]string) (eventID string, ch chan error)
	CaptureMessage(message string, tags map[string]string, interfaces ...raven.Interface) string
	CaptureMessageAndWait(message string, tags map[string]string, interfaces ...raven.Interface) string
	CaptureError(err error, tags map[string]string, interfaces ...raven.Interface) string
	CaptureErrorAndWait(err error, tags map[string]string, interfaces ...raven.Interface) string
	CapturePanic(f func(), tags map[string]string, interfaces ...raven.Interface) (err interface{}, errorID string)
	CapturePanicAndWait(f func(), tags map[string]string, interfaces ...raven.Interface) (err interface{}, errorID string)
	Close()
	Wait()
	URL() string
	ProjectID() string
	Release() string
	IncludePaths() []string
	SetIncludePaths(p []string)
	SetUserContext(u *raven.User)
	SetHttpContext(h *raven.Http)
	SetTagsContext(t map[string]string)
	ClearContext()
}
