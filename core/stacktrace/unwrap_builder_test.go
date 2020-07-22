package stacktrace

import (
	"errors"
	"testing"

	"github.com/getsentry/raven-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// simpleError is a simplest error implementation possible. The only reason why it's here is tests.
type simpleError struct {
	msg string
}

func newSimpleError(msg string) error {
	return &simpleError{msg: msg}
}

func (n *simpleError) Error() string {
	return n.msg
}

// wrappableError is a simple implementation of wrappable error
type wrappableError struct {
	msg string
	err error
}

func newWrappableError(msg string, child error) error {
	return &wrappableError{msg: msg, err: child}
}

func (e *wrappableError) Error() string {
	return e.msg
}

func (e *wrappableError) Unwrap() error {
	return e.err
}

type UnwrapBuilderSuite struct {
	builder *UnwrapBuilder
	suite.Suite
}

func TestUnwrapBuilder(t *testing.T) {
	suite.Run(t, new(UnwrapBuilderSuite))
}

func (s *UnwrapBuilderSuite) SetupTest() {
	client, _ := raven.New("fake dsn")
	s.builder = &UnwrapBuilder{}
	s.builder.SetClient(client)
}

func (s *UnwrapBuilderSuite) TestBuild_Nil() {
	stack, err := s.builder.Build().GetResult()
	require.Error(s.T(), err)
	if stack != nil {
		assert.Empty(s.T(), stack.Frames)
	}
	assert.Equal(s.T(), ErrUnfeasibleBuilder, err)
}

func (s *UnwrapBuilderSuite) TestBuild_NoUnwrap() {
	s.builder.SetError(newSimpleError("fake"))
	stack, buildErr := s.builder.Build().GetResult()
	require.Error(s.T(), buildErr)
	require.Equal(s.T(), ErrUnfeasibleBuilder, buildErr)
	assert.Empty(s.T(), stack)
}

func (s *UnwrapBuilderSuite) TestBuild_WrappableHasWrapped() {
	testErr := newWrappableError("first", newWrappableError("second", errors.New("third")))
	_, ok := testErr.(Unwrappable)
	require.True(s.T(), ok)

	s.builder.SetError(testErr)
	stack, buildErr := s.builder.Build().GetResult()
	require.NoError(s.T(), buildErr)
	require.NotNil(s.T(), stack)
	require.NotNil(s.T(), stack.Frames)
	assert.NotEmpty(s.T(), stack.Frames)
	assert.True(s.T(), len(stack.Frames) > 1)
}
