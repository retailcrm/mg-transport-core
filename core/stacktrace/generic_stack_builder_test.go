package stacktrace

import (
	"testing"

	"github.com/getsentry/raven-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GenericStackBuilderSuite struct {
	builder *GenericStackBuilder
	suite.Suite
}

func TestGenericStack(t *testing.T) {
	suite.Run(t, new(GenericStackBuilderSuite))
}

func (s *GenericStackBuilderSuite) SetupSuite() {
	client, _ := raven.New("fake dsn")
	s.builder = &GenericStackBuilder{AbstractStackBuilder{
		client: client,
	}}
}

func (s *GenericStackBuilderSuite) Test_Build() {
	stack, err := s.builder.Build().GetResult()
	require.Nil(s.T(), err)
	assert.NotEmpty(s.T(), stack)
}
