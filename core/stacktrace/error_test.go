package stacktrace

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ErrorTest struct {
	suite.Suite
}

func TestError(t *testing.T) {
	suite.Run(t, new(ErrorTest))
}

func (t *ErrorTest) TestAppendToError() {
	err := errors.New("test error")
	_, ok := err.(StackTraced)

	t.Assert().False(ok)

	withTrace := AppendToError(err)
	twiceTrace := AppendToError(withTrace)

	t.Assert().Nil(AppendToError(nil))
	t.Assert().Implements((*StackTraced)(nil), withTrace)
	t.Assert().Implements((*StackTraced)(nil), twiceTrace)
	t.Assert().Equal(withTrace.(StackTraced).StackTrace(), twiceTrace.(StackTraced).StackTrace())
}

func (t *ErrorTest) TestCauseUnwrap() {
	err := errors.New("test error")
	wrapped := AppendToError(err)

	t.Assert().Equal(err, wrapped.(*withStack).Cause())
	t.Assert().Equal(err, errors.Unwrap(wrapped))
	t.Assert().Equal(wrapped.(*withStack).Cause(), errors.Unwrap(wrapped))
}

func (t *ErrorTest) TestFormat() {
	wrapped := AppendToError(errors.New("test error"))

	t.Assert().Equal("\""+wrapped.Error()+"\"", fmt.Sprintf("%q", wrapped))
	t.Assert().Equal(wrapped.Error(), fmt.Sprintf("%s", wrapped))
	t.Assert().Equal(wrapped.Error(), fmt.Sprintf("%v", wrapped))
	t.Assert().NotEqual(wrapped.Error(), fmt.Sprintf("%+v", wrapped))
	t.Assert().Contains(fmt.Sprintf("%+v", wrapped), "TestFormat")
}
