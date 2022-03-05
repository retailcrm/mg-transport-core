// Package stacktrace contains code borrowed from the github.com/pkg/errors
package stacktrace

import (
	"fmt"
	"io"
)

// withStack is an error with stacktrace.
type withStack struct {
	error
	*stack
}

// AppendToError populates err with a stack trace at the point WithStack was called.
// If err is nil, WithStack returns nil.
func AppendToError(err error, skip ...int) error {
	if err == nil {
		return nil
	}
	if _, hasTrace := err.(interface {
		StackTrace() StackTrace
	}); hasTrace {
		return err
	}
	framesToSkip := 3
	if len(skip) > 0 {
		framesToSkip = skip[0]
	}
	return &withStack{
		err,
		callers(framesToSkip),
	}
}

// Cause of error.
func (w *withStack) Cause() error { return w.error }

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withStack) Unwrap() error { return w.error }

// Format the error.
func (w *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", w.Cause())
			w.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, w.Error())
	case 'q':
		fmt.Fprintf(s, "%q", w.Error())
	}
}
