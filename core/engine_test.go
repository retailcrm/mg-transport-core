package core

import (
	"bytes"
	"crypto/x509"
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
	"github.com/retailcrm/mg-transport-core/v2/core/middleware"
	"github.com/retailcrm/mg-transport-core/v2/core/util/httputil"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

// TestSentryDSN is a fake Sentry DSN in valid format.
const TestSentryDSN = "https://9f4719e96b0fc2422c05a2f745f214d5@a000000.ingest.sentry.io/0000000"

type EngineTest struct {
	suite.Suite
	engine *Engine
}

type AppInfoTest struct {
	suite.Suite
}

func (e *EngineTest) appInfo() AppInfo {
	return AppInfo{
		Version:   "v0.0",
		Commit:    "commit message",
		Build:     "build",
		BuildDate: "01.01.1970",
	}
}

func (e *EngineTest) SetupTest() {
	var (
		db  *sql.DB
		err error
	)

	e.engine = New(e.appInfo())
	require.NotNil(e.T(), e.engine)

	db, _, err = sqlmock.New()
	require.NoError(e.T(), err)

	createTestLangFiles(e.T())

	e.engine.Config = config.Config{
		Version:  "1",
		LogLevel: 5,
		Database: config.DatabaseConfig{
			Connection:         db,
			Logging:            true,
			TablePrefix:        "",
			MaxOpenConnections: 10,
			MaxIdleConnections: 10,
			ConnectionLifetime: 60,
		},
		SentryDSN: TestSentryDSN,
		HTTPServer: config.HTTPServerConfig{
			Host:   "0.0.0.0",
			Listen: ":3001",
		},
		Debug:          true,
		UpdateInterval: 30,
		ConfigAWS:      config.AWS{},
		TransportInfo: config.Info{
			Name:     "test",
			Code:     "test",
			LogoPath: "/test.svg",
		},
	}
}

func (e *EngineTest) Test_Prepare_Twice() {
	defer func() {
		r := recover()
		require.NotNil(e.T(), r)
		assert.Equal(e.T(), "engine already initialized", r.(string))
	}()

	engine := New(e.appInfo())
	engine.prepared = true
	engine.Prepare()
}

func (e *EngineTest) Test_Prepare_NoConfig() {
	defer func() {
		r := recover()
		require.NotNil(e.T(), r)
		assert.Equal(e.T(), "engine.Config must be loaded before initializing", r.(string))
	}()

	engine := New(e.appInfo())
	engine.prepared = false
	engine.Config = nil
	engine.Prepare()
}

func (e *EngineTest) Test_Prepare() {
	defer func() {
		require.Nil(e.T(), recover())
	}()

	e.engine.TranslationsPath = testTranslationsDir
	e.engine.Prepare()
	assert.True(e.T(), e.engine.prepared)
	assert.NotNil(e.T(), e.engine.Config)
	assert.NotEmpty(e.T(), e.engine.DefaultError)
	assert.NotEmpty(e.T(), e.engine.LogFormatter)
	assert.NotEmpty(e.T(), e.engine.LocaleMatcher)
	assert.False(e.T(), e.engine.isUnd(e.engine.Localizer.LanguageTag))
	assert.NotNil(e.T(), e.engine.DB)
	assert.NotEmpty(e.T(), e.engine.SentryConfig.Dsn)
	assert.NotNil(e.T(), e.engine.logger)
	assert.NotNil(e.T(), e.engine.Sentry.Localizer)
	assert.NotNil(e.T(), e.engine.Sentry.Logger)
	assert.NotNil(e.T(), e.engine.Utils.Logger)
}

func (e *EngineTest) Test_initGin_Release() {
	engine := New(e.appInfo())
	engine.Config = config.Config{Debug: false}
	engine.initGin()
	assert.NotNil(e.T(), engine.ginEngine)
}

func (e *EngineTest) Test_TemplateFuncMap() {
	assert.NotNil(e.T(), e.engine.TemplateFuncMap(template.FuncMap{
		"test": func() string {
			return "test"
		},
	}))
}

func (e *EngineTest) Test_CreateRenderer() {
	e.engine.CreateRenderer(func(r *Renderer) {
		assert.NotNil(e.T(), r)
	}, template.FuncMap{})
}

func (e *EngineTest) Test_Router_Fail() {
	defer func() {
		r := recover()
		require.NotNil(e.T(), r)
		assert.Equal(e.T(), "prepare engine first", r.(string))
	}()

	engine := New(e.appInfo())
	engine.Router()
}

func (e *EngineTest) Test_Router() {
	e.engine.TranslationsPath = testTranslationsDir
	e.engine.Prepare()
	assert.NotNil(e.T(), e.engine.Router())
}

func (e *EngineTest) Test_JobManager() {
	defer func() {
		require.Nil(e.T(), recover())
	}()

	require.Nil(e.T(), e.engine.jobManager)
	manager := e.engine.JobManager()
	require.NotNil(e.T(), manager)
	assert.Equal(e.T(), manager, e.engine.JobManager())
}

func (e *EngineTest) Test_ConfigureRouter() {
	e.engine.TranslationsPath = testTranslationsDir
	e.engine.Prepare()
	e.engine.ConfigureRouter(func(engine *gin.Engine) {
		assert.NotNil(e.T(), engine)
	})
}

func (e *EngineTest) Test_BuildHTTPClient() {
	e.engine.Config = &config.Config{
		HTTPClientConfig: &config.HTTPClientConfig{
			Timeout:         30,
			SSLVerification: boolPtr(true),
		},
	}
	e.engine.BuildHTTPClient(x509.NewCertPool())

	assert.NotNil(e.T(), e.engine.httpClient)
	assert.NotNil(e.T(), e.engine.httpClient.Transport)

	transport := e.engine.httpClient.Transport.(*http.Transport)
	assert.NotNil(e.T(), transport.TLSClientConfig)
	assert.NotNil(e.T(), transport.TLSClientConfig.RootCAs)
}

func (e *EngineTest) Test_BuildHTTPClient_NoConfig() {
	e.engine.Config = &config.Config{}
	e.engine.BuildHTTPClient(x509.NewCertPool())

	assert.NotNil(e.T(), e.engine.httpClient)
	assert.NotNil(e.T(), e.engine.httpClient.Transport)

	transport := e.engine.httpClient.Transport.(*http.Transport)
	assert.NotNil(e.T(), transport.TLSClientConfig)
	assert.NotNil(e.T(), transport.TLSClientConfig.RootCAs)
}

func (e *EngineTest) Test_GetHTTPClientConfig() {
	e.engine.Config = &config.Config{}
	assert.Equal(e.T(), DefaultHTTPClientConfig, e.engine.GetHTTPClientConfig())

	e.engine.Config = &config.Config{
		HTTPClientConfig: &config.HTTPClientConfig{
			Timeout:         10,
			SSLVerification: boolPtr(true),
		},
	}
	assert.NotEqual(e.T(), DefaultHTTPClientConfig, e.engine.GetHTTPClientConfig())
	assert.Equal(e.T(), time.Duration(10), e.engine.GetHTTPClientConfig().Timeout)
}

func (e *EngineTest) Test_WithCookieSessions() {
	e.engine.Sessions = nil
	e.engine.WithCookieSessions(4)

	assert.NotNil(e.T(), e.engine.Sessions)
}

func (e *EngineTest) Test_WithFilesystemSessions() {
	e.engine.Sessions = nil
	e.engine.WithFilesystemSessions(os.TempDir(), 4)

	assert.NotNil(e.T(), e.engine.Sessions)
}

func (e *EngineTest) Test_SetLogger() {
	origLogger := e.engine.logger
	defer func() {
		e.engine.logger = origLogger
	}()
	e.engine.logger = &logger.StandardLogger{}
	e.engine.SetLogger(nil)
	assert.NotNil(e.T(), e.engine.logger)
}

func (e *EngineTest) Test_SetHTTPClient() {
	origClient := e.engine.httpClient
	defer func() {
		e.engine.httpClient = origClient
	}()
	e.engine.httpClient = nil
	httpClient, err := httputil.NewHTTPClientBuilder().Build()
	require.NoError(e.T(), err)
	assert.NotNil(e.T(), httpClient)
	e.engine.SetHTTPClient(&http.Client{})
	require.NotNil(e.T(), e.engine.httpClient)
	e.engine.SetHTTPClient(nil)
	assert.NotNil(e.T(), e.engine.httpClient)
}

func (e *EngineTest) Test_HTTPClient() {
	origClient := e.engine.httpClient
	defer func() {
		e.engine.httpClient = origClient
	}()
	e.engine.httpClient = nil
	require.Same(e.T(), http.DefaultClient, e.engine.HTTPClient())
	httpClient, err := httputil.NewHTTPClientBuilder().Build()
	require.NoError(e.T(), err)
	e.engine.httpClient = httpClient
	assert.Same(e.T(), httpClient, e.engine.HTTPClient())
}

func (e *EngineTest) Test_InitCSRF_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.Sessions = nil
	e.engine.InitCSRF("test", func(context *gin.Context, r middleware.CSRFErrorReason) {}, middleware.DefaultCSRFTokenGetter)
	assert.Nil(e.T(), e.engine.csrf)
}

func (e *EngineTest) Test_InitCSRF() {
	defer func() {
		assert.Nil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.WithCookieSessions(4)
	e.engine.InitCSRF("test", func(context *gin.Context, r middleware.CSRFErrorReason) {}, middleware.DefaultCSRFTokenGetter)
	assert.NotNil(e.T(), e.engine.csrf)
}

func (e *EngineTest) Test_VerifyCSRFMiddleware_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.VerifyCSRFMiddleware(middleware.DefaultIgnoredMethods)
}

func (e *EngineTest) Test_VerifyCSRFMiddleware() {
	defer func() {
		assert.Nil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.WithCookieSessions(4)
	e.engine.InitCSRF("test", func(context *gin.Context, r middleware.CSRFErrorReason) {}, middleware.DefaultCSRFTokenGetter)
	e.engine.VerifyCSRFMiddleware(middleware.DefaultIgnoredMethods)
}

func (e *EngineTest) Test_GenerateCSRFMiddleware_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.GenerateCSRFMiddleware()
}

func (e *EngineTest) Test_GenerateCSRFMiddleware() {
	defer func() {
		assert.Nil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.WithCookieSessions(4)
	e.engine.InitCSRF("test", func(context *gin.Context, r middleware.CSRFErrorReason) {}, middleware.DefaultCSRFTokenGetter)
	e.engine.GenerateCSRFMiddleware()
}

func (e *EngineTest) Test_GetCSRFToken_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.GetCSRFToken(nil)
}

func (e *EngineTest) Test_GetCSRFToken() {
	defer func() {
		assert.Nil(e.T(), recover())
	}()

	c := &gin.Context{Request: &http.Request{
		URL: &url.URL{
			RawQuery: "",
		},
		Body:   ioutil.NopCloser(bytes.NewReader([]byte{})),
		Header: http.Header{"X-CSRF-Token": []string{"token"}},
	}}
	c.Set("csrf_token", "token")

	e.engine.csrf = nil
	e.engine.WithCookieSessions(4)
	e.engine.InitCSRF("test", func(context *gin.Context, r middleware.CSRFErrorReason) {}, middleware.DefaultCSRFTokenGetter)
	assert.NotEmpty(e.T(), e.engine.GetCSRFToken(c))
	assert.Equal(e.T(), "token", e.engine.GetCSRFToken(c))
}

func (e *EngineTest) Test_Run_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	_ = New(e.appInfo()).Run()
}

func (t *AppInfoTest) Test_Release_NoData() {
	a := AppInfo{}

	t.Assert().Equal(
		"<unknown version> (<unknown build>, built <unknown build date>, commit \"<no commit info>\")",
		a.Release())
}

func (t *AppInfoTest) Test_Release() {
	a := AppInfo{
		Version:   "1647352938",
		Commit:    "cb03e2f - replace old Sentry client with the new SDK <Neur0toxine>",
		Build:     "v0.0-cb03e2f",
		BuildDate: "Вт 15 мар 2022 17:03:43 MSK",
	}

	t.Assert().Equal(fmt.Sprintf("%s (%s, built %s, commit \"%s\")", a.Version, a.Build, a.BuildDate, a.Commit),
		a.Release())
}

func TestEngine_Suite(t *testing.T) {
	suite.Run(t, new(EngineTest))
}

func TestAppInfo_Suite(t *testing.T) {
	suite.Run(t, new(AppInfoTest))
}

func boolPtr(val bool) *bool {
	b := val
	return &b
}
