package middleware

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"testing"
)

type SentryMiddlewaresTestSuite struct {
	suite.Suite
}

func TestSentryMiddlewares(t *testing.T) {
	suite.Run(t, new(SentryMiddlewaresTestSuite))
}

func (s *SentryMiddlewaresTestSuite) ctx(mock Sentry) *gin.Context {
	ctx := &gin.Context{}
	InjectSentry(mock)(ctx)
	return ctx
}

func (s *SentryMiddlewaresTestSuite) TestGetSentry_Empty() {
	item, found := GetSentry(&gin.Context{})
	s.Assert().False(found)
	s.Assert().Nil(item)

	item, found = GetSentry(&gin.Context{
		Keys: map[string]interface{}{
			ginContextSentryKey: &gin.Engine{},
		},
	})
	s.Assert().False(found)
	s.Assert().Nil(item)
}

func (s *SentryMiddlewaresTestSuite) TestMustGetSentry_Empty() {
	s.Assert().Panics(func() {
		MustGetSentry(&gin.Context{})
	})
}

func (s *SentryMiddlewaresTestSuite) TestGetSentry_Success() {
	item, found := GetSentry(&gin.Context{
		Keys: map[string]interface{}{
			ginContextSentryKey: &sentryMock{},
		},
	})
	s.Assert().True(found)
	s.Assert().NotNil(item)
}

func (s *SentryMiddlewaresTestSuite) TestMustGetSentry_Success() {
	s.Assert().NotPanics(func() {
		item := MustGetSentry(&gin.Context{
			Keys: map[string]interface{}{
				ginContextSentryKey: &sentryMock{},
			},
		})
		s.Assert().NotNil(item)
	})
}

func (s *SentryMiddlewaresTestSuite) TestCaptureException() {
	err := errors.New("test error")
	item := &sentryMock{}
	item.On("CaptureException", mock.AnythingOfType("*gin.Context"), err).Return()
	CaptureException(s.ctx(item), err)
	item.AssertExpectations(s.T())
}

func (s *SentryMiddlewaresTestSuite) TestCaptureMessage() {
	msg := "test error"
	item := &sentryMock{}
	item.On("CaptureMessage", mock.AnythingOfType("*gin.Context"), msg).Return()
	CaptureMessage(s.ctx(item), msg)
	item.AssertExpectations(s.T())
}

func (s *SentryMiddlewaresTestSuite) TestCaptureEvent() {
	event := &sentry.Event{EventID: "1"}
	item := &sentryMock{}
	item.On("CaptureEvent", mock.AnythingOfType("*gin.Context"), event).Return()
	CaptureEvent(s.ctx(item), event)
	item.AssertExpectations(s.T())
}

type sentryMock struct {
	mock.Mock
}

func (s *sentryMock) CaptureException(c *gin.Context, exception error) {
	s.Called(c, exception)
}

func (s *sentryMock) CaptureMessage(c *gin.Context, message string) {
	s.Called(c, message)
}

func (s *sentryMock) CaptureEvent(c *gin.Context, event *sentry.Event) {
	s.Called(c, event)
}
