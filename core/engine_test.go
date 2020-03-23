package core

import (
	"bytes"
	"database/sql"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EngineTest struct {
	suite.Suite
	engine *Engine
}

func (e *EngineTest) SetupTest() {
	var (
		db  *sql.DB
		err error
	)

	e.engine = New()
	require.NotNil(e.T(), e.engine)

	db, _, err = sqlmock.New()
	require.NoError(e.T(), err)

	createTestLangFiles(e.T())

	e.engine.Config = Config{
		Version:  "1",
		LogLevel: 5,
		Database: DatabaseConfig{
			Connection:         db,
			Logging:            true,
			TablePrefix:        "",
			MaxOpenConnections: 10,
			MaxIdleConnections: 10,
			ConnectionLifetime: 60,
		},
		SentryDSN: "sentry dsn",
		HTTPServer: HTTPServerConfig{
			Host:   "0.0.0.0",
			Listen: ":3001",
		},
		Debug:          true,
		UpdateInterval: 30,
		ConfigAWS:      ConfigAWS{},
		TransportInfo: Info{
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

	engine := New()
	engine.prepared = true
	engine.Prepare()
}

func (e *EngineTest) Test_Prepare_NoConfig() {
	defer func() {
		r := recover()
		require.NotNil(e.T(), r)
		assert.Equal(e.T(), "engine.Config must be loaded before initializing", r.(string))
	}()

	engine := New()
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
}

func (e *EngineTest) Test_initGin_Release() {
	engine := New()
	engine.Config = Config{Debug: false}
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

	engine := New()
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
	e.engine.Config = &Config{
		HTTPClientConfig: &HTTPClientConfig{
			Timeout:         30,
			SSLVerification: boolPtr(true),
		},
	}
	e.engine.BuildHTTPClient()

	assert.NotNil(e.T(), e.engine.httpClient)
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
	e.engine.logger = &Logger{}
	e.engine.SetLogger(nil)
	assert.NotNil(e.T(), e.engine.logger)
}

func (e *EngineTest) Test_SetHTTPClient() {
	origClient := e.engine.httpClient
	defer func() {
		e.engine.httpClient = origClient
	}()
	e.engine.httpClient = nil
	httpClient, err := NewHTTPClientBuilder().Build()
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
	httpClient, err := NewHTTPClientBuilder().Build()
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
	e.engine.InitCSRF("test", func(context *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
	assert.Nil(e.T(), e.engine.csrf)
}

func (e *EngineTest) Test_InitCSRF() {
	defer func() {
		assert.Nil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.WithCookieSessions(4)
	e.engine.InitCSRF("test", func(context *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
	assert.NotNil(e.T(), e.engine.csrf)
}

func (e *EngineTest) Test_VerifyCSRFMiddleware_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.VerifyCSRFMiddleware(DefaultIgnoredMethods)
}

func (e *EngineTest) Test_VerifyCSRFMiddleware() {
	defer func() {
		assert.Nil(e.T(), recover())
	}()

	e.engine.csrf = nil
	e.engine.WithCookieSessions(4)
	e.engine.InitCSRF("test", func(context *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
	e.engine.VerifyCSRFMiddleware(DefaultIgnoredMethods)
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
	e.engine.InitCSRF("test", func(context *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
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
	e.engine.InitCSRF("test", func(context *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
	assert.NotEmpty(e.T(), e.engine.GetCSRFToken(c))
	assert.Equal(e.T(), "token", e.engine.GetCSRFToken(c))
}

func (e *EngineTest) Test_Run_Fail() {
	defer func() {
		assert.NotNil(e.T(), recover())
	}()

	_ = New().Run()
}

func TestEngine_Suite(t *testing.T) {
	suite.Run(t, new(EngineTest))
}

func boolPtr(val bool) *bool {
	b := val
	return &b
}
