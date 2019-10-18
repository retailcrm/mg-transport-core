package core

import (
	"database/sql"
	"html/template"
	"io/ioutil"
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

func (e *EngineTest) SetupSuite() {
	var (
		db  *sql.DB
		err error
	)

	e.engine = New()
	require.NotNil(e.T(), e.engine)

	db, _, err = sqlmock.New()
	require.NoError(e.T(), err)

	if _, err := os.Stat(testTranslationsDir); err != nil && os.IsNotExist(err) {
		err := os.Mkdir(testTranslationsDir, os.ModePerm)
		require.Nil(e.T(), err)
		data := []byte("message: Test message\nmessage_template: Test message with {{.data}}")
		err = ioutil.WriteFile(testLangFile, data, os.ModePerm)
		require.Nil(e.T(), err)
	}

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

func (e *EngineTest) Test_ConfigureRouter() {
	e.engine.TranslationsPath = testTranslationsDir
	e.engine.Prepare()
	e.engine.ConfigureRouter(func(engine *gin.Engine) {
		assert.NotNil(e.T(), engine)
	})
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
