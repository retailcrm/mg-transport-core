package stacktrace

import (
	"github.com/getsentry/raven-go"
)

// Unwrappable is the interface for errors with Unwrap() method.
type Unwrappable interface {
	Unwrap() error
}

// UnwrapBuilder builds stacktrace from the chain of wrapped errors.
type UnwrapBuilder struct {
	AbstractStackBuilder
}

// IsUnwrappableError returns true if error can be unwrapped.
func IsUnwrappableError(err error) bool {
	_, ok := err.(Unwrappable) // nolint:errorlint
	return ok
}

// Build stacktrace.
func (b *UnwrapBuilder) Build() StackBuilderInterface {
	if !IsUnwrappableError(b.err) {
		b.buildErr = ErrUnfeasibleBuilder
		return b
	}

	err := b.err
	var frames []*raven.StacktraceFrame

	for err != nil {
		frames = append(frames, raven.NewStacktraceFrame(
			0,
			"<message>: "+err.Error(),
			"<wrapped>",
			0,
			3,
			b.client.IncludePaths(),
		))

		if item, ok := err.(Unwrappable); ok { // nolint:errorlint
			err = item.Unwrap()
		} else {
			err = nil
		}
	}

	if len(frames) <= 1 {
		b.buildErr = ErrUnfeasibleBuilder
		return b
	}

	// Sentry wants the frames with the oldest first, so reverse them
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}

	b.stack = &raven.Stacktrace{Frames: frames}
	return b
}
