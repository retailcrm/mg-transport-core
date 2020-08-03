package stacktrace

import (
	pkgErrors "github.com/pkg/errors"
)

// PkgErrorCauseable is an interface for checking Cause() method existence in the error
type PkgErrorCauseable interface {
	Cause() error
}

// PkgErrorTraceable is an interface for checking StackTrace() method existence in the error
type PkgErrorTraceable interface {
	StackTrace() pkgErrors.StackTrace
}

// PkgErrorsStackTransformer transforms stack data from github.com/pkg/errors error to stacktrace.Stacktrace
type PkgErrorsStackTransformer struct {
	stack pkgErrors.StackTrace
}

// NewPkgErrorsStackTransformer is a PkgErrorsStackTransformer constructor
func NewPkgErrorsStackTransformer(stack pkgErrors.StackTrace) *PkgErrorsStackTransformer {
	return &PkgErrorsStackTransformer{stack: stack}
}

// Stack returns stacktrace (which is []uintptr internally, each uintptc is a pc)
func (p *PkgErrorsStackTransformer) Stack() Stacktrace {
	if p.stack == nil {
		return Stacktrace{}
	}

	result := make(Stacktrace, len(p.stack))
	for i, frame := range p.stack {
		result[i] = Frame(uintptr(frame) - 1)
	}
	return result
}

// PkgErrorsBuilder builds stacktrace with data from github.com/pkg/errors error
type PkgErrorsBuilder struct {
	AbstractStackBuilder
}

// Build stacktrace
func (b *PkgErrorsBuilder) Build() StackBuilderInterface {
	if !isPkgErrors(b.err) {
		b.buildErr = ErrUnfeasibleBuilder
		return b
	}

	var stack pkgErrors.StackTrace
	err := b.err

	for err != nil {
		s := b.getErrorStack(err)
		if s != nil {
			stack = s
		}
		err = b.getErrorCause(err)
	}

	if len(stack) > 0 {
		b.stack = NewRavenStacktraceBuilder(NewPkgErrorsStackTransformer(stack)).Build(3, b.client.IncludePaths())
	} else {
		b.buildErr = ErrUnfeasibleBuilder
	}

	return b
}

// getErrorCause will try to extract original error from wrapper - it is used only if stacktrace is not present
func (b *PkgErrorsBuilder) getErrorCause(err error) error {
	causeable, ok := err.(PkgErrorCauseable)
	if !ok {
		return nil
	}
	return causeable.Cause()
}

// getErrorStackTrace will try to extract stacktrace from error using StackTrace method (default errors doesn't have it)
func (b *PkgErrorsBuilder) getErrorStack(err error) pkgErrors.StackTrace {
	traceable, ok := err.(PkgErrorTraceable)
	if !ok {
		return nil
	}
	return traceable.StackTrace()
}
