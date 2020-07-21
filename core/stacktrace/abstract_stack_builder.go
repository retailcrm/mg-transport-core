package stacktrace

import (
	"errors"

	"github.com/getsentry/raven-go"
)

var UnfeasibleBuilder = errors.New("unfeasible builder for this error type")

type StackBuilderInterface interface {
	SetClient(RavenClientInterface) StackBuilderInterface
	SetError(error) StackBuilderInterface
	Build() StackBuilderInterface
	GetResult() (*raven.Stacktrace, error)
}

type AbstractStackBuilder struct {
	err      error
	buildErr error
	client   RavenClientInterface
	stack    *raven.Stacktrace
}

func (a *AbstractStackBuilder) SetClient(client RavenClientInterface) StackBuilderInterface {
	a.client = client
	return a
}

func (a *AbstractStackBuilder) SetError(err error) StackBuilderInterface {
	a.err = err
	return a
}

func (a *AbstractStackBuilder) Build() StackBuilderInterface {
	panic("not implemented")
}

func (a *AbstractStackBuilder) GetResult() (*raven.Stacktrace, error) {
	return a.stack, a.buildErr
}
