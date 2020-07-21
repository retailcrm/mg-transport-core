package stacktrace

import (
	"path"
	"runtime"

	"github.com/getsentry/raven-go"
)

// Frame is a program counter inside a stack frame.
type Frame uintptr

// Stacktrace is stack of Frames
type Stacktrace []Frame

// RavenStackProvider is an interface for any component, which will provide stacktrace data
type RavenStackProvider interface {
	Stack() Stacktrace
}

// RavenStacktraceBuilder builds *raven.Stacktrace for any generic stack data
type RavenStacktraceBuilder struct {
	provider RavenStackProvider
}

// NewRavenStacktraceBuilder is a RavenStacktraceBuilder constructor
func NewRavenStacktraceBuilder(p RavenStackProvider) *RavenStacktraceBuilder {
	return (&RavenStacktraceBuilder{}).SetProvider(p)
}

// SetProvider sets provider into stacktrace builder
func (b *RavenStacktraceBuilder) SetProvider(p RavenStackProvider) *RavenStacktraceBuilder {
	b.provider = p
	return b
}

// Build converts generic stacktrace to to github.com/getsentry/raven-go.Stacktrace
func (b *RavenStacktraceBuilder) Build(context int, appPackagePrefixes []string) *raven.Stacktrace {
	// This code is borrowed from github.com/getsentry/raven-go.NewStacktrace().
	var frames []*raven.StacktraceFrame
	for _, f := range b.provider.Stack() {
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

// convertFrame converts single generic stacktrace frame to github.com/pkg/errors.Frame
func (b *RavenStacktraceBuilder) convertFrame(f Frame, context int, appPackagePrefixes []string) *raven.StacktraceFrame {
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
