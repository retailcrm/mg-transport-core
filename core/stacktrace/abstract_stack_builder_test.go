package stacktrace

import (
	"errors"
	"testing"

	"github.com/getsentry/raven-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AbstractStackBuilderSuite struct {
	builder *AbstractStackBuilder
	suite.Suite
}

func TestAbstractStackBuilder(t *testing.T) {
	suite.Run(t, new(AbstractStackBuilderSuite))
}

func (s *AbstractStackBuilderSuite) SetupSuite() {
	s.builder = &AbstractStackBuilder{}
}

func (s *AbstractStackBuilderSuite) Test_SetClient() {
	require.Nil(s.T(), s.builder.client)
	client, _ := raven.New("fake dsn")
	s.builder.SetClient(client)
	assert.NotNil(s.T(), s.builder.client)
}

func (s *AbstractStackBuilderSuite) Test_SetError() {
	require.Nil(s.T(), s.builder.err)
	s.builder.SetError(errors.New("test err"))
	assert.NotNil(s.T(), s.builder.err)
}

func (s *AbstractStackBuilderSuite) Test_Build() {
	defer func() {
		r := recover()
		require.NotNil(s.T(), r)
		require.IsType(s.T(), "", r)
		assert.Equal(s.T(), "not implemented", r.(string))
	}()

	s.builder.Build()
}

func (s *AbstractStackBuilderSuite) Test_GetResult() {
	buildErr := errors.New("build err")
	stack := raven.NewStacktrace(0, 3, []string{})
	s.builder.buildErr = buildErr
	s.builder.stack = stack
	resultStack, resultErr := s.builder.GetResult()

	assert.Error(s.T(), resultErr)
	assert.Equal(s.T(), buildErr, resultErr)
	assert.Equal(s.T(), *stack, *resultStack)
}
