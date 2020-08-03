package stacktrace

import (
	"github.com/getsentry/raven-go"
)

// GenericStackBuilder uses raven.NewStacktrace to build stacktrace. Only client is needed here.
type GenericStackBuilder struct {
	AbstractStackBuilder
}

// Build returns generic stacktrace.
func (b *GenericStackBuilder) Build() StackBuilderInterface {
	b.stack = GenericStack(b.client)
	return b
}

// GenericStack returns generic stacktrace.
func GenericStack(client RavenClientInterface) *raven.Stacktrace {
	return raven.NewStacktrace(0, 3, client.IncludePaths())
}
