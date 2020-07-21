package stacktrace

import (
	"github.com/getsentry/raven-go"
)

type GenericStackBuilder struct {
	AbstractStackBuilder
}

func (b *GenericStackBuilder) Build() StackBuilderInterface {
	b.stack = GenericStack(b.client)
	return b
}

func GenericStack(client RavenClientInterface) *raven.Stacktrace {
	return raven.NewStacktrace(0, 3, client.IncludePaths())
}
