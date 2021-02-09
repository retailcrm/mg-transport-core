package core

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type sampleStruct struct {
	ID      int
	Pointer *int
	Field   string
}

type ravenPacket struct {
	EventID    string
	Message    string
	Tags       map[string]string
	Interfaces []raven.Interface
}

func (r ravenPacket) getInterface(class string) (raven.Interface, bool) {
	for _, v := range r.Interfaces {
		if v.Class() == class {
			return v, true
		}
	}

	return nil, false
}

func (r ravenPacket) getException() (*raven.Exception, bool) {
	if i, ok := r.getInterface("exception"); ok {
		if r, ok := i.(*raven.Exception); ok {
			return r, true
		}
	}

	return nil, false
}

func (r ravenPacket) getRequest() (*raven.Http, bool) {
	if i, ok := r.getInterface("request"); ok {
		if r, ok := i.(*raven.Http); ok {
			return r, true
		}
	}

	return nil, false
}

type ravenClientMock struct {
	raven.Client
	captured []ravenPacket
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

func newRavenMock() *ravenClientMock {
	rand.Seed(time.Now().UnixNano())
	return &ravenClientMock{captured: []ravenPacket{}}
}

func (r *ravenClientMock) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.captured = []ravenPacket{}
}

func (r *ravenClientMock) last() (ravenPacket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.captured) > 0 {
		return r.captured[len(r.captured)-1], nil
	}

	return ravenPacket{}, errors.New("empty packet list")
}

func (r *ravenClientMock) CaptureMessageAndWait(message string, tags map[string]string, interfaces ...raven.Interface) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.wg.Done()
	eventID := strconv.FormatUint(rand.Uint64(), 10)
	r.captured = append(r.captured, ravenPacket{
		EventID:    eventID,
		Message:    message,
		Tags:       tags,
		Interfaces: interfaces,
	})
	return eventID
}

func (r *ravenClientMock) CaptureErrorAndWait(err error, tags map[string]string, interfaces ...raven.Interface) string {
	return r.CaptureMessageAndWait(err.Error(), tags, interfaces...)
}

func (r *ravenClientMock) IncludePaths() []string {
	return []string{}
}

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

// wrappableError is a simple implementation of wrappable error.
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

type SentryTest struct {
	suite.Suite
	sentry     *Sentry
	gin        *gin.Engine
	structTags *SentryTaggedStruct
	scalarTags *SentryTaggedScalar
}

func (s *SentryTest) SetupSuite() {
	s.structTags = NewTaggedStruct(sampleStruct{}, "struct", map[string]string{"fake": "prop"})
	s.scalarTags = NewTaggedScalar("", "scalar", "Scalar")
	require.Equal(s.T(), "struct", s.structTags.GetContextKey())
	require.Equal(s.T(), "scalar", s.scalarTags.GetContextKey())
	require.Equal(s.T(), "", s.structTags.GetName())
	require.Equal(s.T(), "Scalar", s.scalarTags.GetName())
	s.structTags.Tags = map[string]string{}
	s.sentry = NewSentry("dsn", "unknown_error", SentryTaggedTypes{}, nil, nil)
	s.sentry.Client = newRavenMock()
	s.gin = gin.New()
	s.gin.Use(s.sentry.ErrorMiddleware())
}

func (s *SentryTest) TestStruct_AddTag() {
	s.structTags.AddTag("test field", "Field")
	require.NotEmpty(s.T(), s.structTags.GetTags())

	tags, err := s.structTags.BuildTags(sampleStruct{Field: "value"})
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), tags)

	i, ok := tags["test field"]
	require.True(s.T(), ok)
	assert.Equal(s.T(), "value", i)
}

func (s *SentryTest) TestStruct_GetProperty() {
	s.structTags.AddTag("test field", "Field")
	name, value, err := s.structTags.GetProperty(sampleStruct{Field: "test"}, "Field")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "test field", name)
	assert.Equal(s.T(), "test", value)
}

func (s *SentryTest) TestStruct_GetProperty_InvalidStruct() {
	_, _, err := s.structTags.GetProperty(nil, "Field")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "invalid value provided", err.Error())
}

func (s *SentryTest) TestStruct_GetProperty_GotScalar() {
	_, _, err := s.structTags.GetProperty("", "Field")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "passed value must be struct, string provided", err.Error())
}

func (s *SentryTest) TestStruct_GetProperty_InvalidType() {
	_, _, err := s.structTags.GetProperty(Sentry{}, "Field")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "passed value should be of type `core.sampleStruct`, got `core.Sentry` instead", err.Error())
}

func (s *SentryTest) TestStruct_GetProperty_CannotFindProperty() {
	_, _, err := s.structTags.GetProperty(sampleStruct{ID: 1}, "ID")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "cannot find property `ID`", err.Error())
}

func (s *SentryTest) TestStruct_GetProperty_InvalidProperty() {
	s.structTags.AddTag("test invalid", "Pointer")
	_, _, err := s.structTags.GetProperty(sampleStruct{Pointer: nil}, "Pointer")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "invalid property, got <invalid Value>", err.Error())
}

func (s *SentryTest) TestStruct_BuildTags_Fail() {
	s.structTags.Tags = map[string]string{}
	s.structTags.AddTag("test", "Field")
	_, err := s.structTags.BuildTags(false)
	assert.Error(s.T(), err)
}

func (s *SentryTest) TestStruct_BuildTags() {
	s.structTags.Tags = map[string]string{}
	s.structTags.AddTag("test", "Field")
	tags, err := s.structTags.BuildTags(sampleStruct{Field: "value"})

	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), tags)
	i, ok := tags["test"]
	require.True(s.T(), ok)
	assert.Equal(s.T(), "value", i)
}

func (s *SentryTest) TestScalar_Get_Nil() {
	_, err := s.scalarTags.Get(nil)
	require.Error(s.T(), err)
	assert.Equal(s.T(), "invalid value provided", err.Error())
}

func (s *SentryTest) TestScalar_Get_Struct() {
	_, err := s.scalarTags.Get(struct{}{})
	require.Error(s.T(), err)
	assert.Equal(s.T(), "passed value must not be struct", err.Error())
}

func (s *SentryTest) TestScalar_Get_InvalidType() {
	_, err := s.scalarTags.Get(false)
	require.Error(s.T(), err)
	assert.Equal(s.T(), "passed value should be of type `string`, got `bool` instead", err.Error())
}

func (s *SentryTest) TestScalar_Get() {
	val, err := s.scalarTags.Get("test")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "test", val)
}

func (s *SentryTest) TestScalar_GetTags() {
	assert.Empty(s.T(), s.scalarTags.GetTags())
}

func (s *SentryTest) TestScalar_BuildTags_Fail() {
	_, err := s.scalarTags.BuildTags(false)
	assert.Error(s.T(), err)
}

func (s *SentryTest) TestScalar_BuildTags() {
	tags, err := s.scalarTags.BuildTags("test")

	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), tags)
	i, ok := tags[s.scalarTags.GetName()]
	require.True(s.T(), ok)
	assert.Equal(s.T(), "test", i)
}

func (s *SentryTest) TestSentry_ErrorMiddleware() {
	assert.NotNil(s.T(), s.sentry.ErrorMiddleware())
}

func (s *SentryTest) TestSentry_PanicLogger() {
	assert.NotNil(s.T(), s.sentry.PanicLogger())
}

func (s *SentryTest) TestSentry_ErrorLogger() {
	assert.NotNil(s.T(), s.sentry.ErrorLogger())
}

func (s *SentryTest) TestSentry_ErrorResponseHandler() {
	assert.NotNil(s.T(), s.sentry.ErrorResponseHandler())
}

func (s *SentryTest) TestSentry_ErrorCaptureHandler() {
	assert.NotNil(s.T(), s.sentry.ErrorCaptureHandler())
}

func (s *SentryTest) TestSentry_CaptureRegularError() {
	s.gin.GET("/test_regularError", func(c *gin.Context) {
		c.Error(newSimpleError("test"))
	})

	var resp ErrorsResponse
	req, err := http.NewRequest(http.MethodGet, "/test_regularError", nil)
	require.NoError(s.T(), err)

	ravenMock := s.sentry.Client.(*ravenClientMock)
	ravenMock.wg.Add(1)
	rec := httptest.NewRecorder()
	s.gin.ServeHTTP(rec, req)
	require.NoError(s.T(), json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(s.T(), resp.Error)
	assert.Equal(s.T(), s.sentry.DefaultError, resp.Error[0])

	ravenMock.wg.Wait()
	last, err := ravenMock.last()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "test", last.Message)

	exception, ok := last.getException()
	require.True(s.T(), ok, "cannot find exception")
	require.NotNil(s.T(), exception.Stacktrace)
	assert.NotEmpty(s.T(), exception.Stacktrace.Frames)
}

// TestSentry_CaptureWrappedError is used to check if Sentry component calls stacktrace builders properly
// Actual stacktrace builder tests can be found in the corresponding package.
func (s *SentryTest) TestSentry_CaptureWrappedError() {
	third := newWrappableError("third", nil)
	second := newWrappableError("second", third)
	first := newWrappableError("first", second)

	s.gin.GET("/test_wrappableError", func(c *gin.Context) {
		c.Error(first)
	})

	var resp ErrorsResponse
	req, err := http.NewRequest(http.MethodGet, "/test_wrappableError", nil)
	require.NoError(s.T(), err)

	ravenMock := s.sentry.Client.(*ravenClientMock)
	ravenMock.wg.Add(1)
	rec := httptest.NewRecorder()
	s.gin.ServeHTTP(rec, req)
	require.NoError(s.T(), json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(s.T(), resp.Error)
	assert.Equal(s.T(), s.sentry.DefaultError, resp.Error[0])

	ravenMock.wg.Wait()
	last, err := ravenMock.last()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "first", last.Message)

	exception, ok := last.getException()
	require.True(s.T(), ok, "cannot find exception")
	require.NotNil(s.T(), exception.Stacktrace)
	assert.NotEmpty(s.T(), exception.Stacktrace.Frames)
	assert.Len(s.T(), exception.Stacktrace.Frames, 3)

	// Error messages will be put into function names by parser
	assert.Contains(s.T(), exception.Stacktrace.Frames[0].Function, third.Error())
	assert.Contains(s.T(), exception.Stacktrace.Frames[1].Function, second.Error())
	assert.Contains(s.T(), exception.Stacktrace.Frames[2].Function, first.Error())
}

func (s *SentryTest) TestSentry_CaptureTags() {
	s.gin.GET("/test_taggedError", func(c *gin.Context) {
		var intPointer = 147
		c.Set("text_tag", "text contents")
		c.Set("sample_struct", sampleStruct{
			ID:      12,
			Pointer: &intPointer,
			Field:   "field content",
		})
	}, func(c *gin.Context) {
		c.Error(newSimpleError("test"))
	})

	s.sentry.TaggedTypes = SentryTaggedTypes{
		NewTaggedScalar("", "text_tag", "TextTag"),
		NewTaggedStruct(sampleStruct{}, "sample_struct", map[string]string{
			"id":         "ID",
			"pointer":    "Pointer",
			"field item": "Field",
		}),
	}

	var resp ErrorsResponse
	req, err := http.NewRequest(http.MethodGet, "/test_taggedError", nil)
	require.NoError(s.T(), err)

	ravenMock := s.sentry.Client.(*ravenClientMock)
	ravenMock.wg.Add(1)
	rec := httptest.NewRecorder()
	s.gin.ServeHTTP(rec, req)
	require.NoError(s.T(), json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(s.T(), resp.Error)
	assert.Equal(s.T(), s.sentry.DefaultError, resp.Error[0])

	ravenMock.wg.Wait()
	last, err := ravenMock.last()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "test", last.Message)

	exception, ok := last.getException()
	require.True(s.T(), ok, "cannot find exception")
	require.NotNil(s.T(), exception.Stacktrace)
	assert.NotEmpty(s.T(), exception.Stacktrace.Frames)

	// endpoint tag is present by default
	require.NotEmpty(s.T(), last.Tags)
	assert.True(s.T(), len(last.Tags) == 5)
	assert.Equal(s.T(), "text contents", last.Tags["TextTag"])
	assert.Equal(s.T(), "12", last.Tags["id"])
	assert.Equal(s.T(), "147", last.Tags["pointer"])
	assert.Equal(s.T(), "field content", last.Tags["field item"])
}

func TestSentry_Suite(t *testing.T) {
	suite.Run(t, new(SentryTest))
}
