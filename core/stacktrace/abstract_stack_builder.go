package stacktrace

import (
	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
)

// UnfeasibleBuilder will be returned if builder for stacktrace was chosen incorrectly
var UnfeasibleBuilder = errors.New("unfeasible builder for this error type")

// StackBuilderInterface is an interface for every stacktrace builder
type StackBuilderInterface interface {
	SetClient(RavenClientInterface) StackBuilderInterface
	SetError(error) StackBuilderInterface
	Build() StackBuilderInterface
	GetResult() (*raven.Stacktrace, error)
}

// AbstractStackBuilder contains methods, which would be implemented in every builder anyway
type AbstractStackBuilder struct {
	err      error
	buildErr error
	client   RavenClientInterface
	stack    *raven.Stacktrace
}

// SetClient sets *raven.Client into builder. RavenClientInterface is used, so, any client might be used via facade.
func (a *AbstractStackBuilder) SetClient(client RavenClientInterface) StackBuilderInterface {
	a.client = client
	return a
}

// SetError sets error in builder, which will be processed
func (a *AbstractStackBuilder) SetError(err error) StackBuilderInterface {
	a.err = err
	return a
}

// Build stacktrace. Only implemented in the children.
func (a *AbstractStackBuilder) Build() StackBuilderInterface {
	panic("not implemented")
}

// GetResult returns builder result.
func (a *AbstractStackBuilder) GetResult() (*raven.Stacktrace, error) {
	return a.stack, a.buildErr
}

// FallbackToGeneric fallbacks to GenericStackBuilder method
func (a *AbstractStackBuilder) FallbackToGeneric() {
	a.stack, a.err = GenericStack(a.client), nil
}
