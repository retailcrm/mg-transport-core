package stacktrace

import (
	"path/filepath"

	"github.com/getsentry/raven-go"

	"github.com/retailcrm/mg-transport-core/core/errortools"
)

// ErrorNodesList is the interface for the error.errList.
type ErrorNodesList interface {
	Iterate() <-chan errortools.Node
	Len() int
}

// ErrCollectorBuilder builds stacktrace from the list of errors collected by error.Collector.
type ErrCollectorBuilder struct {
	AbstractStackBuilder
}

// IsErrorNodesList returns true if error contains error nodes.
func IsErrorNodesList(err error) bool {
	_, ok := err.(ErrorNodesList) // nolint:errorlint
	return ok
}

// AsErrorNodesList returns ErrorNodesList instance from the error.
func AsErrorNodesList(err error) ErrorNodesList {
	return err.(ErrorNodesList) // nolint:errorlint
}

// Build stacktrace.
func (b *ErrCollectorBuilder) Build() StackBuilderInterface {
	if !IsErrorNodesList(b.err) {
		b.buildErr = ErrUnfeasibleBuilder
		return b
	}

	i := 0
	errs := AsErrorNodesList(b.err)
	frames := make([]*raven.StacktraceFrame, errs.Len())

	for err := range errs.Iterate() {
		frames[i] = raven.NewStacktraceFrame(
			err.PC, filepath.Base(err.File), err.File, err.Line, 3, b.client.IncludePaths())
		i++
	}

	if len(frames) <= 1 {
		b.buildErr = ErrUnfeasibleBuilder
		return b
	}

	b.stack = &raven.Stacktrace{Frames: frames}
	return b
}
