package stacktrace

import (
	"path"
	"runtime"

	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
)

type PkgErrorsBuilder struct {
	AbstractStackBuilder
}

func (b *PkgErrorsBuilder) Build() StackBuilderInterface {
	var stack errors.StackTrace
	err := b.err

	for err != nil {
		s := b.getErrorStack(err)
		if s != nil {
			stack = s
		}
		err = b.getErrorCause(err)
	}

	if len(stack) > 0 {
		b.stack = b.convertStack(stack, 3, b.client.IncludePaths())
	} else {
		b.buildErr = UnfeasibleBuilder
	}

	return b
}

// getErrorCause will try to extract original error from wrapper - it is used only if stacktrace is not present
func (b *PkgErrorsBuilder) getErrorCause(err error) error {
	causeable, ok := err.(interface {
		Cause() error
	})
	if !ok {
		return nil
	}
	return causeable.Cause()
}

// getErrorStackTrace will try to extract stacktrace from error using StackTrace method (default errors doesn't have it)
func (b *PkgErrorsBuilder) getErrorStack(err error) errors.StackTrace {
	traceable, ok := err.(interface {
		StackTrace() errors.StackTrace
	})
	if !ok {
		return nil
	}
	return traceable.StackTrace()
}

// convertStackTrace converts github.com/pkg/errors.StackTrace to github.com/getsentry/raven-go.Stacktrace
func (b *PkgErrorsBuilder) convertStack(st errors.StackTrace, context int, appPackagePrefixes []string) *raven.Stacktrace {
	// This code is borrowed from github.com/getsentry/raven-go.NewStacktrace().
	var frames []*raven.StacktraceFrame
	for _, f := range st {
		frame := b.convertFrame(f, context, appPackagePrefixes)
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
func (b *PkgErrorsBuilder) convertFrame(f errors.Frame, context int, appPackagePrefixes []string) *raven.StacktraceFrame {
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
	return raven.NewStacktraceFrame(pc, path.Dir(file), file, line, context, appPackagePrefixes)
}
