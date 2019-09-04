package internal

import (
	"runtime"

	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
)

// NewRavenStackTrace generate stacktrace compatible with raven-go format
// It tries to extract better stacktrace from error from package "github.com/pkg/errors"
// In case of fail it will fallback to default stacktrace generation from raven-go.
// Default stacktrace highly likely will be useless, because it will not include call
// which returned error. This occurs because default stacktrace doesn't include any call
// before stacktrace generation, and raven-go will generate stacktrace here, which will end
// in trace to this file. But errors from "github.com/pkg/errors" will generate stacktrace
// immediately, it will include call which returned error, and we can fetch this trace.
// Also we can wrap default errors with error from this package, like this:
//      errors.Wrap(err, err.Error)
func NewRavenStackTrace(client *raven.Client, myerr error, skip int) *raven.Stacktrace {
	st := getErrorStackTraceConverted(myerr, 3, client.IncludePaths())
	if st == nil {
		st = raven.NewStacktrace(skip, 3, client.IncludePaths())
	}
	return st
}

// getErrorStackTraceConverted will return converted stacktrace from custom error, or nil in case of default error
func getErrorStackTraceConverted(err error, context int, appPackagePrefixes []string) *raven.Stacktrace {
	st := getErrorCauseStackTrace(err)
	if st == nil {
		return nil
	}
	return convertStackTrace(st, context, appPackagePrefixes)
}

// getErrorCauseStackTrace tries to extract stacktrace from custom error, returns nil in case of failure
func getErrorCauseStackTrace(err error) errors.StackTrace {
	// This code is inspired by github.com/pkg/errors.Cause().
	var st errors.StackTrace
	for err != nil {
		s := getErrorStackTrace(err)
		if s != nil {
			st = s
		}
		err = getErrorCause(err)
	}
	return st
}

// convertStackTrace converts github.com/pkg/errors.StackTrace to github.com/getsentry/raven-go.Stacktrace
func convertStackTrace(st errors.StackTrace, context int, appPackagePrefixes []string) *raven.Stacktrace {
	// This code is borrowed from github.com/getsentry/raven-go.NewStacktrace().
	var frames []*raven.StacktraceFrame
	for _, f := range st {
		frame := convertFrame(f, context, appPackagePrefixes)
		if frame != nil {
			frames = append(frames, frame)
		}
	}
	if len(frames) == 0 {
		return nil
	}
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}
	return &raven.Stacktrace{Frames: frames}
}

// convertFrame converts single frame from github.com/pkg/errors.Frame to github.com/pkg/errors.Frame
func convertFrame(f errors.Frame, context int, appPackagePrefixes []string) *raven.StacktraceFrame {
	// This code is borrowed from github.com/pkg/errors.Frame.
	pc := uintptr(f) - 1
	fn := runtime.FuncForPC(pc)
	var file string
	var line int
	if fn != nil {
		file, line = fn.FileLine(pc)
	} else {
		file = "unknown"
	}
	return raven.NewStacktraceFrame(pc, file, line, context, appPackagePrefixes)
}

// getErrorStackTrace will try to extract stacktrace from error using StackTrace method (default errors doesn't have it)
func getErrorStackTrace(err error) errors.StackTrace {
	ster, ok := err.(interface {
		StackTrace() errors.StackTrace
	})
	if !ok {
		return nil
	}
	return ster.StackTrace()
}

// getErrorCause will try to extract original error from wrapper - it is used only if stacktrace is not present
func getErrorCause(err error) error {
	cer, ok := err.(interface {
		Cause() error
	})
	if !ok {
		return nil
	}
	return cer.Cause()
}
