package core

import (
	"crypto/x509"
	"embed"
	"html/template"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/op/go-logging"
	"golang.org/x/text/language"
)

var boolTrue = true

// DefaultHTTPClientConfig is a default config for HTTP client. It will be used by Engine for building HTTP client
// if HTTP client config is not present in the configuration.
var DefaultHTTPClientConfig = &HTTPClientConfig{
	Timeout:         30,
	SSLVerification: &boolTrue,
}

// Engine struct.
type Engine struct {
	Localizer
	ORM
	Sentry
	Utils
	ginEngine        *gin.Engine
	httpClient       *http.Client
	logger           LoggerInterface
	mutex            sync.RWMutex
	csrf             *CSRF
	jobManager       *JobManager
	PreloadLanguages []language.Tag
	Sessions         sessions.Store
	Config           ConfigInterface
	LogFormatter     logging.Formatter
	prepared         bool
}

// New Engine instance (must be configured manually, gin can be accessed via engine.Router() directly or
// engine.ConfigureRouter(...) with callback).
func New() *Engine {
	return &Engine{
		Config: nil,
		Localizer: Localizer{
			i18nStorage: &sync.Map{},
			loadMutex:   &sync.RWMutex{},
		},
		PreloadLanguages: []language.Tag{},
		ORM:              ORM{},
		Sentry:           Sentry{},
		Utils:            Utils{},
		ginEngine:        nil,
		logger:           nil,
		mutex:            sync.RWMutex{},
		prepared:         false,
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

// Prepare engine for start.
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
	if e.LocaleMatcher == nil {
		e.LocaleMatcher = DefaultLocalizerMatcher()
	}

	if e.isUnd(e.Localizer.LanguageTag) {
		e.Localizer.LanguageTag = DefaultLanguage
	}

	e.LoadTranslations()

	if len(e.PreloadLanguages) > 0 {
		e.Localizer.Preload(e.PreloadLanguages)
	}

	e.createDB(e.Config.GetDBConfig())
	e.createRavenClient(e.Config.GetSentryDSN())
	e.resetUtils(e.Config.GetAWSConfig(), e.Config.IsDebug(), 0)
	e.SetLogger(NewLogger(e.Config.GetTransportInfo().GetCode(), e.Config.GetLogLevel(), e.LogFormatter))
	e.Sentry.Localizer = &e.Localizer
	e.Sentry.Stacktrace = true
	e.Utils.Logger = e.Logger()
	e.Sentry.Logger = e.Logger()
	e.prepared = true

	return e
}

// TemplateFuncMap combines func map for templates.
func (e *Engine) TemplateFuncMap(functions template.FuncMap) template.FuncMap {
	funcMap := e.LocalizationFuncMap()

	for name, fn := range functions {
		funcMap[name] = fn
	}

	funcMap["version"] = func() string {
		return e.Config.GetVersion()
	}

	return funcMap
}

// CreateRenderer with translation function.
func (e *Engine) CreateRenderer(callback func(*Renderer), funcs template.FuncMap) Renderer {
	renderer := NewRenderer(e.TemplateFuncMap(funcs))
	callback(&renderer)
	return renderer
}

// CreateRendererFS with translation function and embedded files.
func (e *Engine) CreateRendererFS(
	templatesFS embed.FS,
	templatesDir string,
	callback func(*Renderer),
	funcs template.FuncMap,
) Renderer {
	renderer := NewRenderer(e.TemplateFuncMap(funcs))
	renderer.TemplatesFS = templatesFS
	renderer.TemplatesDir = templatesDir
	callback(&renderer)
	return renderer
}

// Router will return current gin.Engine or panic if it's not present.
func (e *Engine) Router() *gin.Engine {
	if !e.prepared {
		panic("prepare engine first")
	}
	if e.ginEngine == nil {
		e.initGin()
	}

	return e.ginEngine
}

// JobManager will return singleton JobManager from Engine.
func (e *Engine) JobManager() *JobManager {
	if e.jobManager == nil {
		e.jobManager = NewJobManager().SetLogger(e.Logger()).SetLogging(e.Config.IsDebug())
	}

	return e.jobManager
}

// Logger returns current logger.
func (e *Engine) Logger() LoggerInterface {
	return e.logger
}

// SetLogger sets provided logger instance to engine.
func (e *Engine) SetLogger(l LoggerInterface) *Engine {
	if l == nil {
		return e
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.logger = l
	return e
}

// BuildHTTPClient builds HTTP client with provided configuration.
func (e *Engine) BuildHTTPClient(certs *x509.CertPool, replaceDefault ...bool) *Engine {
	client, err := NewHTTPClientBuilder().
		WithLogger(e.Logger()).
		SetLogging(e.Config.IsDebug()).
		SetCertPool(certs).
		FromEngine(e).
		Build(replaceDefault...)

	if err != nil {
		panic(err)
	} else {
		e.httpClient = client
	}

	return e
}

// GetHTTPClientConfig returns configuration for HTTP client.
func (e *Engine) GetHTTPClientConfig() *HTTPClientConfig {
	if e.Config.GetHTTPClientConfig() != nil {
		return e.Config.GetHTTPClientConfig()
	}

	return DefaultHTTPClientConfig
}

// SetHTTPClient sets HTTP client to engine.
func (e *Engine) SetHTTPClient(client *http.Client) *Engine {
	if client != nil {
		e.httpClient = client
	}

	return e
}

// HTTPClient returns inner http client or default http client.
func (e *Engine) HTTPClient() *http.Client {
	if e.httpClient == nil {
		return http.DefaultClient
	}

	return e.httpClient
}

// WithCookieSessions generates new CookieStore with optional key length.
// Default key length is 32 bytes.
func (e *Engine) WithCookieSessions(keyLength ...int) *Engine {
	length := 32

	if len(keyLength) > 0 && keyLength[0] > 0 {
		length = keyLength[0]
	}

	e.Sessions = sessions.NewCookieStore(securecookie.GenerateRandomKey(length))
	return e
}

// WithFilesystemSessions generates new FilesystemStore with optional key length.
// Default key length is 32 bytes.
func (e *Engine) WithFilesystemSessions(path string, keyLength ...int) *Engine {
	length := 32

	if len(keyLength) > 0 && keyLength[0] > 0 {
		length = keyLength[0]
	}

	e.Sessions = sessions.NewFilesystemStore(path, securecookie.GenerateRandomKey(length))
	return e
}

// InitCSRF initializes CSRF middleware. engine.Sessions must be already initialized,
// use engine.WithCookieStore or engine.WithFilesystemStore for that.
// Syntax is similar to core.NewCSRF, but you shouldn't pass sessionName, store and salt.
func (e *Engine) InitCSRF(secret string, abortFunc CSRFAbortFunc, getter CSRFTokenGetter) *Engine {
	if e.Sessions == nil {
		panic("engine.Sessions must be initialized first")
	}

	e.csrf = NewCSRF("", secret, "", e.Sessions, abortFunc, getter)
	return e
}

// VerifyCSRFMiddleware returns CSRF verifier middleware
// Usage:
// 		engine.Router().Use(engine.VerifyCSRFMiddleware(core.DefaultIgnoredMethods))
func (e *Engine) VerifyCSRFMiddleware(ignoredMethods []string) gin.HandlerFunc {
	if e.csrf == nil {
		panic("csrf is not initialized")
	}

	return e.csrf.VerifyCSRFMiddleware(ignoredMethods)
}

// GenerateCSRFMiddleware returns CSRF generator middleware
// Usage:
// 		engine.Router().Use(engine.GenerateCSRFMiddleware())
func (e *Engine) GenerateCSRFMiddleware() gin.HandlerFunc {
	if e.csrf == nil {
		panic("csrf is not initialized")
	}

	return e.csrf.GenerateCSRFMiddleware()
}

// GetCSRFToken returns CSRF token from provided context.
func (e *Engine) GetCSRFToken(c *gin.Context) string {
	if e.csrf == nil {
		panic("csrf is not initialized")
	}

	return e.csrf.CSRFFromContext(c)
}

// ConfigureRouter will call provided callback with current gin.Engine, or panic if engine is not present.
func (e *Engine) ConfigureRouter(callback func(*gin.Engine)) *Engine {
	callback(e.Router())
	return e
}

// Run gin.Engine loop, or panic if engine is not present.
func (e *Engine) Run() error {
	return e.Router().Run(e.Config.GetHTTPConfig().Listen)
}
