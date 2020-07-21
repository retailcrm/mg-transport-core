package stacktrace

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ravenMockTransformer struct {
	mock.Mock
}

func (r *ravenMockTransformer) Stack() Stacktrace {
	args := r.Called()
	return args.Get(0).(Stacktrace)
}

type RavenStacktraceBuilderSuite struct {
	suite.Suite
}

func (s *RavenStacktraceBuilderSuite) callers() Stacktrace {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	st := make(Stacktrace, n)
	for i := 0; i < n; i++ {
		st[i] = Frame(pcs[i])
	}
	return st
}

func (s *RavenStacktraceBuilderSuite) Test_BuildEmpty() {
	testTransformer := new(ravenMockTransformer)
	testTransformer.On("Stack", mock.Anything).Return(Stacktrace{})

	assert.Nil(s.T(), NewRavenStacktraceBuilder(testProvider).Build(3, []string{}))
}

func (s *RavenStacktraceBuilderSuite) Test_BuildActual() {
	testTransformer := new(ravenMockTransformer)
	testTransformer.On("Stack", mock.Anything).Return(s.callers())
	stack := NewRavenStacktraceBuilder(testProvider).Build(3, []string{})

	require.NotNil(s.T(), stack)
	assert.NotEmpty(s.T(), stack.Frames)
}

func TestRavenStacktraceBuilder(t *testing.T) {
	suite.Run(t, new(RavenStacktraceBuilderSuite))
}
