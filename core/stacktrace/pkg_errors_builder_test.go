package stacktrace

import (
	"errors"
	"testing"

	"github.com/getsentry/raven-go"
	pkgErrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// errorWithCause has Cause() method, but doesn't have StackTrace() method
type errorWithCause struct {
	msg   string
	cause error
}

func newErrorWithCause(msg string, cause error) error {
	return &errorWithCause{
		msg:   msg,
		cause: cause,
	}
}

func (e *errorWithCause) Error() string {
	return e.msg
}

func (e *errorWithCause) Cause() error {
	return e.cause
}

type PkgErrorsStackProviderSuite struct {
	transformer *PkgErrorsStackTransformer
	suite.Suite
}

func (s *PkgErrorsStackProviderSuite) SetupSuite() {
	s.transformer = &PkgErrorsStackTransformer{}
}

func (s *PkgErrorsStackProviderSuite) Test_Nil() {
	s.transformer.stack = nil
	assert.Empty(s.T(), s.transformer.Stack())
}

func (s *PkgErrorsStackProviderSuite) Test_Empty() {
	s.transformer.stack = pkgErrors.StackTrace{}
	assert.Empty(s.T(), s.transformer.Stack())
}

func (s *PkgErrorsStackProviderSuite) Test_Full() {
	testErr := pkgErrors.New("test")
	s.transformer.stack = testErr.(PkgErrorTraceable).StackTrace()
	assert.NotEmpty(s.T(), s.transformer.Stack())
}

type PkgErrorsBuilderSuite struct {
	builder *PkgErrorsBuilder
	suite.Suite
}

func (s *PkgErrorsBuilderSuite) SetupTest() {
	s.builder = &PkgErrorsBuilder{}
	client, _ := raven.New("fake dsn")
	s.builder.SetClient(client)
}

func (s *PkgErrorsBuilderSuite) Test_Stackless() {
	s.builder.SetError(errors.New("simple"))
	stack, err := s.builder.Build().GetResult()
	require.Error(s.T(), err)
	assert.Equal(s.T(), ErrUnfeasibleBuilder, err)
	assert.Empty(s.T(), stack)
}

func (s *PkgErrorsBuilderSuite) Test_WithStack() {
	s.builder.SetError(pkgErrors.New("with stack"))
	stack, err := s.builder.Build().GetResult()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), stack)
	assert.NotEmpty(s.T(), stack.Frames)
}

func (s *PkgErrorsBuilderSuite) Test_CauseWithStack() {
	s.builder.SetError(newErrorWithCause("cause with stack", pkgErrors.New("with stack")))
	stack, err := s.builder.Build().GetResult()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), stack)
	assert.NotEmpty(s.T(), stack.Frames)
}

func TestPkgErrorsStackProvider(t *testing.T) {
	suite.Run(t, new(PkgErrorsStackProviderSuite))
}

func TestPkgErrorsBuilder(t *testing.T) {
	suite.Run(t, new(PkgErrorsBuilderSuite))
}
