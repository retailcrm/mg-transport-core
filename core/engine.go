package core

import (
	"crypto/x509"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/blacked/go-zabbix"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/retailcrm/zabbix-metrics-collector"
	"golang.org/x/text/language"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
	"github.com/retailcrm/mg-transport-core/v2/core/db"
	"github.com/retailcrm/mg-transport-core/v2/core/middleware"
	"github.com/retailcrm/mg-transport-core/v2/core/util"
	"github.com/retailcrm/mg-transport-core/v2/core/util/httputil"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

var boolTrue = true

// DefaultHTTPClientConfig is a default config for HTTP client. It will be used by Engine for building HTTP client
// if HTTP client config is not present in the configuration.
var DefaultHTTPClientConfig = &config.HTTPClientConfig{
	Timeout:         30,
	SSLVerification: &boolTrue,
}

// AppInfo contains information about app version.
type AppInfo struct {
	Version   string
	Commit    string
	Build     string
	BuildDate string
}

// Release information for Sentry.
func (a AppInfo) Release() string {
	if a.Version == "" {
		a.Version = "<unknown version>"
	}
	if a.Build == "" {
		a.Build = "<unknown build>"
	}
	if a.BuildDate == "" {
		a.BuildDate = "<unknown build date>"
	}
	if a.Commit == "" {
		a.Commit = "<no commit info>"
	}
	return fmt.Sprintf("%s (%s, built %s, commit \"%s\")", a.Version, a.Build, a.BuildDate, a.Commit)
}

// Engine struct.
type Engine struct {
	logger     logger.Logger
	AppInfo    AppInfo
	Sessions   sessions.Store
	Config     config.Configuration
	Zabbix     metrics.Transport
	ginEngine  *gin.Engine
	csrf       *middleware.CSRF
	httpClient *http.Client
	jobManager *JobManager
	db.ORM
	Localizer
	util.Utils
	PreloadLanguages []language.Tag
	Sentry
	mutex    sync.RWMutex
	prepared bool
}

// New Engine instance (must be configured manually, gin can be accessed via engine.Router() directly or
// engine.ConfigureRouter(...) with callback).
func New(appInfo AppInfo) *Engine {
	return &Engine{
		Config:  nil,
		AppInfo: appInfo,
		Localizer: Localizer{
			i18nStorage: &sync.Map{},
			loadMutex:   &sync.RWMutex{},
		},
		PreloadLanguages: []language.Tag{},
		ORM:              db.ORM{},
		Sentry:           Sentry{},
		Utils:            util.Utils{},
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

	e.buildSentryConfig()
	e.InitSentrySDK()
	r.Use(e.SentryMiddlewares()...)

	if e.Config.IsDebug() {
		r.Use(gin.Logger())
	}

	r.Use(e.LocalizationMiddleware())
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
	if e.LocaleMatcher == nil {
		e.LocaleMatcher = DefaultLocalizerMatcher()
	}

	if e.isUnd(e.Localizer.Language()) {
		e.Localizer.LanguageTag = DefaultLanguage
	}

	e.LoadTranslations()

	if len(e.PreloadLanguages) > 0 {
		e.Localizer.Preload(e.PreloadLanguages)
	}

	if !e.Config.IsDebug() {
		logger.DefaultOpts.Level = slog.LevelInfo
	}

	e.CreateDB(e.Config.GetDBConfig())
	e.ResetUtils(e.Config.GetAWSConfig(), e.Config.IsDebug(), 0)
	e.SetLogger(logger.NewDefaultText())
	e.Sentry.Localizer = &e.Localizer
	e.Utils.Logger = e.Logger()
	e.Sentry.Logger = e.Logger()
	e.buildSentryConfig()
	e.Sentry.InitSentrySDK()
	e.prepared = true

	return e
}

func (e *Engine) UseZabbix(collectors []metrics.Collector) *Engine {
	if e.Config == nil || e.Config.GetZabbixConfig().Interval == 0 {
		return e
	}
	if e.Zabbix != nil {
		for _, col := range collectors {
			e.Zabbix.WithCollector(col)
		}
		return e
	}
	cfg := e.Config.GetZabbixConfig()
	sender := zabbix.NewSender(cfg.ServerHost, cfg.ServerPort)
	e.Zabbix = metrics.NewZabbix(collectors, sender, cfg.Host, cfg.Interval, logger.ZabbixCollectorAdapter(e.Logger()))
	return e
}

func (e *Engine) HijackGinLogs() *Engine {
	if e.Logger() == nil {
		return e
	}
	gin.DefaultWriter = logger.WriterAdapter(e.Logger(), slog.LevelDebug)
	gin.DefaultErrorWriter = logger.WriterAdapter(e.Logger(), slog.LevelError)
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		e.Logger().Debug("route",
			slog.String(logger.HTTPMethodAttr, httpMethod),
			slog.String("path", absolutePath),
			slog.String(logger.HandlerAttr, handlerName),
			slog.Int("handlerCount", nuHandlers))
	}
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
	templatesFS fs.FS,
	callback func(*Renderer),
	funcs template.FuncMap,
) Renderer {
	renderer := NewRenderer(e.TemplateFuncMap(funcs))
	renderer.TemplatesFS = templatesFS
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
func (e *Engine) Logger() logger.Logger {
	return e.logger
}

// SetLogger sets provided logger instance to engine.
func (e *Engine) SetLogger(l logger.Logger) *Engine {
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
	client, err := httputil.NewHTTPClientBuilder().
		WithLogger(e.Logger()).
		SetLogging(e.Config.IsDebug()).
		SetCertPool(certs).
		FromConfig(e.GetHTTPClientConfig()).
		Build(replaceDefault...)

	if err != nil {
		panic(err)
	} else {
		e.httpClient = client
	}

	return e
}

// GetHTTPClientConfig returns configuration for HTTP client.
func (e *Engine) GetHTTPClientConfig() *config.HTTPClientConfig {
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
func (e *Engine) InitCSRF(
	secret string, abortFunc middleware.CSRFAbortFunc, getter middleware.CSRFTokenGetter) *Engine {
	if e.Sessions == nil {
		panic("engine.Sessions must be initialized first")
	}

	e.csrf = middleware.NewCSRF("", secret, "", e.Sessions, abortFunc, getter)
	return e
}

// VerifyCSRFMiddleware returns CSRF verifier middleware
// Usage:
//
//	engine.Router().Use(engine.VerifyCSRFMiddleware(core.DefaultIgnoredMethods))
func (e *Engine) VerifyCSRFMiddleware(ignoredMethods []string) gin.HandlerFunc {
	if e.csrf == nil {
		panic("csrf is not initialized")
	}

	return e.csrf.VerifyCSRFMiddleware(ignoredMethods)
}

// GenerateCSRFMiddleware returns CSRF generator middleware
// Usage:
//
//	engine.Router().Use(engine.GenerateCSRFMiddleware())
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
	if e.Zabbix != nil {
		go e.Zabbix.Run()
	}
	return e.Router().Run(e.Config.GetHTTPConfig().Listen)
}

// buildSentryConfig from app configuration.
func (e *Engine) buildSentryConfig() {
	if e.AppInfo.Version == "" {
		e.AppInfo.Version = e.Config.GetVersion()
	}
	e.SentryConfig = sentry.ClientOptions{
		Dsn:              e.Config.GetSentryDSN(),
		ServerName:       e.Config.GetHTTPConfig().Host,
		Release:          e.AppInfo.Release(),
		AttachStacktrace: true,
		Debug:            e.Config.IsDebug(),
	}
}
