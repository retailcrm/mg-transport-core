package core

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/retailcrm/mg-transport-core/v2/core/db/models"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/retailcrm/mg-transport-core/v2/core/stacktrace"
	"github.com/retailcrm/mg-transport-core/v2/core/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type sampleStruct struct {
	Pointer *int
	Field   string
	ID      int
}

type sentryMockTransport struct {
	sending   sync.RWMutex
	lastEvent *sentry.Event
}

func (s *sentryMockTransport) Flush(timeout time.Duration) bool {
	// noop
	return true
}

func (s *sentryMockTransport) Configure(options sentry.ClientOptions) {
	// noop
}

func (s *sentryMockTransport) SendEvent(event *sentry.Event) {
	defer s.sending.Unlock()
	s.sending.Lock()
	s.lastEvent = event
}

func newSentryMockTransport() *sentryMockTransport {
	return &sentryMockTransport{}
}

type SentryTest struct {
	suite.Suite
	logger     testutil.BufferedLogger
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
	s.logger = testutil.NewBufferedLogger()
	appInfo := AppInfo{
		Version:   "test_version",
		Commit:    "test_commit",
		Build:     "test_build",
		BuildDate: "test_build_date",
	}
	s.sentry = &Sentry{
		init: sync.Once{},
		SentryConfig: sentry.ClientOptions{
			Dsn:              TestSentryDSN,
			Debug:            true,
			AttachStacktrace: true,
			Release:          appInfo.Release(),
		},
		ServerName: "test",
		AppInfo:    appInfo,
		Logger:     s.logger,
		SentryLoggerConfig: SentryLoggerConfig{
			TagForConnection: "url",
			TagForAccount:    "name",
		},
		Localizer:    nil,
		DefaultError: "error_save",
		TaggedTypes: SentryTaggedTypes{
			NewTaggedStruct(models.Connection{}, "connection", map[string]string{
				"url": "URL",
			}),
			NewTaggedStruct(models.Account{}, "account", map[string]string{
				"name": "Name",
			}),
		},
	}
	s.sentry.InitSentrySDK()
	s.gin = gin.New()
	s.gin.Use(s.sentry.SentryMiddlewares()...)
}

func (s *SentryTest) hubMock() (hub *sentry.Hub, transport *sentryMockTransport) {
	client, err := sentry.NewClient(s.sentry.SentryConfig)
	if err != nil {
		panic(err)
	}
	transport = newSentryMockTransport()
	client.Transport = transport
	hub = sentry.NewHub(client, sentry.NewScope())
	return
}

func (s *SentryTest) ginCtxMock() (ctx *gin.Context, transport *sentryMockTransport) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	ctx = &gin.Context{Request: req}
	hub, transport := s.hubMock()
	ctx.Set("sentry", hub)
	return
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
	assert.NotNil(s.T(), s.sentry.SentryMiddlewares())
	assert.NotEmpty(s.T(), s.sentry.SentryMiddlewares())
}

func (s *SentryTest) TestSentry_CaptureException_Nil() {
	defer func() {
		s.Assert().Nil(recover())
	}()
	s.sentry.CaptureException(&gin.Context{}, nil)
}

func (s *SentryTest) TestSentry_CaptureException_Error() {
	ctx, transport := s.ginCtxMock()
	ctx.Keys = make(map[string]interface{})
	s.sentry.CaptureException(ctx, errors.New("test error"))

	s.Require().Nil(transport.lastEvent)
	s.Require().Len(ctx.Errors, 1)
}

func (s *SentryTest) TestSentry_CaptureException() {
	ctx, transport := s.ginCtxMock()
	s.sentry.CaptureException(ctx, stacktrace.AppendToError(errors.New("test error")))

	s.Require().NotNil(transport.lastEvent)
	s.Require().Equal(
		"test_version (test_build, built test_build_date, commit \"test_commit\")", transport.lastEvent.Release)
	s.Require().Len(transport.lastEvent.Exception, 2)
	s.Assert().Equal(transport.lastEvent.Exception[0].Type, "*errors.errorString")
	s.Assert().Equal(transport.lastEvent.Exception[0].Value, "test error")
	s.Assert().Nil(transport.lastEvent.Exception[0].Stacktrace)
	s.Assert().Equal(transport.lastEvent.Exception[1].Type, "*stacktrace.withStack")
	s.Assert().Equal(transport.lastEvent.Exception[1].Value, "test error")
	s.Assert().NotNil(transport.lastEvent.Exception[1].Stacktrace)
}

func (s *SentryTest) TestSentry_obtainErrorLogger_Existing() {
	ctx, _ := s.ginCtxMock()
	log := logger.DecorateForAccount(testutil.NewBufferedLogger(), "component", "conn", "acc")
	ctx.Set("logger", log)

	s.Assert().Equal(log, s.sentry.obtainErrorLogger(ctx))
}

func (s *SentryTest) TestSentry_obtainErrorLogger_Constructed() {
	ctx, _ := s.ginCtxMock()
	ctx.Set("connection", &models.Connection{URL: "conn_url"})
	ctx.Set("account", &models.Account{Name: "acc_name"})

	s.sentry.SentryLoggerConfig = SentryLoggerConfig{}
	logNoConfig := s.sentry.obtainErrorLogger(ctx)
	s.sentry.SentryLoggerConfig = SentryLoggerConfig{
		TagForConnection: "url",
		TagForAccount:    "name",
	}
	log := s.sentry.obtainErrorLogger(ctx)

	s.Assert().NotNil(log)
	s.Assert().NotNil(logNoConfig)
	s.Assert().Implements((*logger.AccountLogger)(nil), log)
	s.Assert().Implements((*logger.AccountLogger)(nil), logNoConfig)
	s.Assert().Equal(
		fmt.Sprintf(logger.DefaultAccountLoggerFormat, "Sentry", "{no connection ID}", "{no account ID}"),
		logNoConfig.Prefix())
	s.Assert().Equal(fmt.Sprintf(logger.DefaultAccountLoggerFormat, "Sentry", "conn_url", "acc_name"), log.Prefix())
}

func (s *SentryTest) TestSentry_tagsSetterMiddleware() {
	ctx, transport := s.ginCtxMock()
	ctx.Set("connection", &models.Connection{URL: "conn_url"})
	ctx.Set("account", &models.Account{Name: "acc_name"})

	s.sentry.tagsSetterMiddleware()(ctx)

	hub := sentry.GetHubFromContext(ctx)
	s.Require().NotNil(hub)

	hub.CaptureException(errors.New("test error"))

	s.Require().NotNil(transport.lastEvent)
	s.Require().Len(transport.lastEvent.Exception, 1)
}

func TestSentry_Suite(t *testing.T) {
	suite.Run(t, new(SentryTest))
}
