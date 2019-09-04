package core

import (
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

// Engine struct
type Engine struct {
	Localizer
	ORM
	Sentry
	Utils
	ginEngine    *gin.Engine
	Logger       *logging.Logger
	Config       ConfigInterface
	LogFormatter logging.Formatter
	prepared     bool
}

// New Engine instance (must be configured manually, gin can be accessed via engine.Router() directly or engine.ConfigureRouter(...) with callback)
func New() *Engine {
	return &Engine{
		Config:    nil,
		Localizer: Localizer{},
		ORM:       ORM{},
		Sentry:    Sentry{},
		Utils:     Utils{},
		ginEngine: nil,
		Logger:    nil,
		prepared:  false,
	}
}

func (e *Engine) initGin() {
	if !e.Config.IsDebug() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	if e.Config.IsDebug() {
		r.Use(gin.Logger())
	}

	r.Use(e.LocalizationMiddleware(), e.ErrorMiddleware())
	e.ginEngine = r
}

// Prepare engine for start
func (e *Engine) Prepare() *Engine {
	if e.prepared {
		panic("engine already initialized")
	}
	if e.Config == nil {
		panic("engine.Config must be loaded before initializing")
	}

	if e.DefaultError == "" {
		e.DefaultError = "error"
	}
	if e.LogFormatter == nil {
		e.LogFormatter = DefaultLogFormatter()
	}
	if e.LocaleBundle == nil {
		e.LocaleBundle = DefaultLocalizerBundle()
	}
	if e.LocaleMatcher == nil {
		e.LocaleMatcher = DefaultLocalizerMatcher()
	}

	e.LoadTranslations()
	e.createDB(e.Config.GetDBConfig())
	e.createRavenClient(e.Config.GetSentryDSN())
	e.resetUtils(e.Config.GetAWSConfig(), e.Config.IsDebug(), 0)
	e.Logger = NewLogger(e.Config.GetTransportInfo().GetCode(), e.Config.GetLogLevel(), e.LogFormatter)
	e.Utils.Localizer = &e.Localizer
	e.Sentry.Localizer = &e.Localizer
	e.Utils.Logger = e.Logger
	e.Sentry.Logger = e.Logger
	e.prepared = true

	return e
}

// CreateRenderer with translation function
func (e *Engine) CreateRenderer(callback func(*Renderer)) Renderer {
	renderer := NewRenderer(e.LocalizationFuncMap())
	callback(&renderer)
	return renderer
}

// Router will return current gin.Engine or panic if it's not present
func (e *Engine) Router() *gin.Engine {
	if !e.prepared {
		panic("prepare engine first")
	}
	if e.ginEngine == nil {
		e.initGin()
	}

	return e.ginEngine
}

// ConfigureRouter will call provided callback with current gin.Engine, or panic if engine is not present
func (e *Engine) ConfigureRouter(callback func(*gin.Engine)) *Engine {
	callback(e.Router())
	return e
}

// Run gin.Engine loop, or panic if engine is not present
func (e *Engine) Run() error {
	return e.Router().Run(e.Config.GetHTTPConfig().Listen)
}
