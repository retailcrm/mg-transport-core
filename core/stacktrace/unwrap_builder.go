package stacktrace

import (
	"github.com/getsentry/raven-go"
)

// Unwrappable is the interface for errors with Unwrap() method
type Unwrappable interface {
	Unwrap() error
}

// UnwrapBuilder builds stacktrace from the chain of wrapped errors
type UnwrapBuilder struct {
	AbstractStackBuilder
}

// Build stacktrace
func (b *UnwrapBuilder) Build() StackBuilderInterface {
	if _, ok := b.err.(Unwrappable); !ok {
		b.buildErr = ErrUnfeasibleBuilder
		return b
	}

	err := b.err
	frames := []*raven.StacktraceFrame{}

	for err != nil {
		frames = append(frames, raven.NewStacktraceFrame(
			0,
			"<wrapped>",
			"<wrapped>",
			0,
			3,
			b.client.IncludePaths(),
		))

		if item, ok := err.(Unwrappable); ok {
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
