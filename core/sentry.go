package core

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/pkg/errors"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/retailcrm/mg-transport-core/v2/core/stacktrace"

	"github.com/gin-gonic/gin"
)

// reset is borrowed directly from the gin.
const reset = "\033[0m"

// ErrorHandlerFunc will handle errors.
type ErrorHandlerFunc func(recovery interface{}, c *gin.Context)

// SentryTaggedTypes list.
type SentryTaggedTypes []SentryTagged

// SentryTags list for SentryTaggedStruct. Format: name => property name.
type SentryTags map[string]string

// SentryTagged interface for both tagged scalar and struct.
type SentryTagged interface {
	BuildTags(interface{}) (map[string]string, error)
	GetContextKey() string
	GetTags() SentryTags
	GetName() string
}

// Sentry struct. Holds SentryTaggedStruct list.
type Sentry struct {
	SentryConfig       sentry.ClientOptions
	Logger             logger.Logger
	Localizer          *Localizer
	AppInfo            AppInfo
	SentryLoggerConfig SentryLoggerConfig
	ServerName         string
	DefaultError       string
	TaggedTypes        SentryTaggedTypes
	init               sync.Once
}

// SentryTaggedStruct holds information about type, it's key in gin.Context (for middleware), and it's properties.
type SentryTaggedStruct struct {
	Type          reflect.Type
	Tags          SentryTags
	GinContextKey string
}

// SentryTaggedScalar variable from context.
type SentryTaggedScalar struct {
	SentryTaggedStruct
	Name string
}

// SentryLoggerConfig configures how Sentry component will create account-scoped logger for recovery.
type SentryLoggerConfig struct {
	TagForConnection string
	TagForAccount    string
}

// sentryTag contains sentry tag name and corresponding value from context.
type sentryTag struct {
	Name  string
	Value string
}

// InitSentrySDK globally in the app. Only works once per component (you really shouldn't call this twice).
func (s *Sentry) InitSentrySDK() {
	s.init.Do(func() {
		if err := sentry.Init(s.SentryConfig); err != nil {
			panic(err)
		}
	})
}

// NewTaggedStruct constructor.
func NewTaggedStruct(sample interface{}, ginCtxKey string, tags map[string]string) *SentryTaggedStruct {
	n := make(map[string]string)
	for k, v := range tags {
		n[v] = k
	}

	return &SentryTaggedStruct{
		Type:          reflect.TypeOf(sample),
		GinContextKey: ginCtxKey,
		Tags:          n,
	}
}

// NewTaggedScalar constructor.
func NewTaggedScalar(sample interface{}, ginCtxKey string, name string) *SentryTaggedScalar {
	return &SentryTaggedScalar{
		SentryTaggedStruct: SentryTaggedStruct{
			Type:          reflect.TypeOf(sample),
			GinContextKey: ginCtxKey,
			Tags:          SentryTags{},
		},
		Name: name,
	}
}

// CaptureException and send it to Sentry.
// Use stacktrace.ErrorWithStack to append the stacktrace to the errors without it!
func (s *Sentry) CaptureException(c *gin.Context, exception error) {
	if exception == nil {
		return
	}
	if hub := sentrygin.GetHubFromContext(c); hub != nil {
		s.setScopeTags(c, hub.Scope())
		hub.CaptureException(exception)
		return
	}
	_ = c.Error(exception)
}

// SentryMiddlewares contain all the middlewares required to process errors and panics and send them to the Sentry.
// It also logs those with account identifiers.
func (s *Sentry) SentryMiddlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		s.tagsSetterMiddleware(),
		s.exceptionCaptureMiddleware(),
		s.recoveryMiddleware(),
		sentrygin.New(sentrygin.Options{Repanic: true}),
	}
}

// obtainErrorLogger extracts logger from the context or builds it right here from tags used in Sentry events
// Those tags can be configured with SentryLoggerConfig field.
func (s *Sentry) obtainErrorLogger(c *gin.Context) logger.AccountLogger {
	if item, ok := c.Get("logger"); ok {
		if accountLogger, ok := item.(logger.AccountLogger); ok {
			return accountLogger
		}
	}

	connectionID := "{no connection ID}"
	accountID := "{no account ID}"
	if s.SentryLoggerConfig.TagForConnection == "" && s.SentryLoggerConfig.TagForAccount == "" {
		return logger.DecorateForAccount(s.Logger, "Sentry", connectionID, accountID)
	}

	for tag := range s.tagsFromContext(c) {
		if s.SentryLoggerConfig.TagForConnection != "" && s.SentryLoggerConfig.TagForConnection == tag.Name {
			connectionID = tag.Value
		}
		if s.SentryLoggerConfig.TagForAccount != "" && s.SentryLoggerConfig.TagForAccount == tag.Name {
			accountID = tag.Value
		}
	}

	return logger.DecorateForAccount(s.Logger, "Sentry", connectionID, accountID)
}

// tagsSetterMiddleware sets event tags into Sentry events.
func (s *Sentry) tagsSetterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if hub := sentry.GetHubFromContext(c.Request.Context()); hub != nil {
			s.setScopeTags(c, hub.Scope())
		}
	}
}

// exceptionCaptureMiddleware captures exceptions and sends a proper JSON response for them.
func (s *Sentry) exceptionCaptureMiddleware() gin.HandlerFunc { // nolint:gocognit
	return func(c *gin.Context) {
		defer func() {
			recovery := recover()
			publicErrors := c.Errors.ByType(gin.ErrorTypePublic)
			privateErrors := c.Errors.ByType(gin.ErrorTypePrivate)
			publicLen := len(publicErrors)
			privateLen := len(privateErrors)

			if privateLen == 0 && publicLen == 0 && recovery == nil {
				return
			}

			messagesLen := publicLen
			if privateLen > 0 || recovery != nil {
				messagesLen++
			}

			l := s.obtainErrorLogger(c)
			messages := make([]string, messagesLen)
			index := 0
			for _, err := range publicErrors {
				messages[index] = err.Error()
				s.CaptureException(c, err)
				l.Error(err)
				index++
			}

			for _, err := range privateErrors {
				s.CaptureException(c, err)
				l.Error(err)
			}

			if privateLen > 0 || recovery != nil {
				if s.Localizer == nil {
					messages[index] = s.DefaultError
				} else {
					messages[index] = s.Localizer.GetLocalizedMessage(s.DefaultError)
				}
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": messages})

			// will be caught by Sentry middleware
			if recovery != nil {
				panic(recovery)
			}
		}()

		c.Next()
	}
}

// recoveryMiddleware is mostly borrowed from the gin itself. It only contains several modifications to add logger
// prefixes to all newlines in the log. The amount of changes is infinitesimal in comparison to the original code.
func (s *Sentry) recoveryMiddleware() gin.HandlerFunc { // nolint
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil { // nolint:nestif
				l := s.obtainErrorLogger(c)

				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok { // nolint:errorlint
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}
				if l != nil {
					stack := stacktrace.FormattedStack(3, l.Prefix()+" ")
					formattedErr := fmt.Sprintf("%s %s", l.Prefix(), err)
					httpRequest, _ := httputil.DumpRequest(c.Request, false)
					headers := strings.Split(string(httpRequest), "\r\n")
					for idx, header := range headers {
						current := strings.Split(header, ":")
						if current[0] == "Authorization" {
							headers[idx] = current[0] + ": *"
						}
						headers[idx] = l.Prefix() + " " + headers[idx]
					}
					headersToStr := strings.Join(headers, "\r\n")
					switch {
					case brokenPipe:
						l.Errorf("%s\n%s%s", formattedErr, headersToStr, reset)
					case gin.IsDebugging():
						l.Errorf("[Recovery] %s panic recovered:\n%s\n%s\n%s%s",
							timeFormat(time.Now()), headersToStr, formattedErr, stack, reset)
					default:
						l.Errorf("[Recovery] %s panic recovered:\n%s\n%s%s",
							timeFormat(time.Now()), formattedErr, stack, reset)
					}
				}
				if brokenPipe {
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
				} else {
					if s.Localizer == nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": []string{s.DefaultError}})
						return
					}

					c.JSON(http.StatusInternalServerError, gin.H{
						"error": []string{s.Localizer.GetLocalizedMessage(s.DefaultError)},
					})
				}
			}
		}()
		c.Next()
	}
}

// setScopeTags sets Sentry tags into scope using component configuration.
func (s *Sentry) setScopeTags(c *gin.Context, scope *sentry.Scope) {
	scope.SetTag("endpoint", c.Request.RequestURI)

	for tag := range s.tagsFromContext(c) {
		scope.SetTag(tag.Name, tag.Value)
	}
}

// tagsFromContext extracts tags from context using component configuration.
func (s *Sentry) tagsFromContext(c *gin.Context) chan sentryTag {
	ch := make(chan sentryTag)

	go func(ch chan sentryTag) {
		if len(s.TaggedTypes) > 0 {
			for _, tagged := range s.TaggedTypes {
				if item, ok := c.Get(tagged.GetContextKey()); ok && item != nil {
					if itemTags, err := tagged.BuildTags(item); err == nil {
						for tagName, tagValue := range itemTags {
							ch <- sentryTag{
								Name:  tagName,
								Value: tagValue,
							}
						}
					}
				}
			}
		}

		close(ch)
	}(ch)

	return ch
}

// AddTag will add tag with property name which holds tag in object.
func (t *SentryTaggedStruct) AddTag(name string, property string) *SentryTaggedStruct {
	t.Tags[property] = name
	return t
}

// GetTags is Tags getter.
func (t *SentryTaggedStruct) GetTags() SentryTags {
	return t.Tags
}

// GetContextKey is GinContextKey getter.
func (t *SentryTaggedStruct) GetContextKey() string {
	return t.GinContextKey
}

// GetName is useless for SentryTaggedStruct.
func (t *SentryTaggedStruct) GetName() string {
	return ""
}

// GetProperty will extract property string representation from specified object. It will be properly formatted.
func (t *SentryTaggedStruct) GetProperty(v interface{}, property string) (name string, value string, err error) {
	val := reflect.Indirect(reflect.ValueOf(v))
	if !val.IsValid() {
		err = errors.New("invalid value provided")
		return
	}

	if val.Kind() != reflect.Struct {
		err = fmt.Errorf("passed value must be struct, %s provided", val.Type().String())
		return
	}

	if val.Type().Name() != t.Type.Name() {
		err = fmt.Errorf("passed value should be of type `%s`, got `%s` instead", t.Type.String(), val.Type().String())
		return
	}

	if i, ok := t.Tags[property]; ok {
		name = i
	} else {
		err = fmt.Errorf("cannot find property `%s`", property)
	}

	field := reflect.Indirect(val.FieldByName(property))
	if !field.IsValid() {
		err = fmt.Errorf("invalid property, got %s", field.String())
		return
	}

	value = t.valueToString(field)
	return
}

// BuildTags will extract tags for Sentry from specified object.
func (t *SentryTaggedStruct) BuildTags(v interface{}) (tags map[string]string, err error) {
	items := make(map[string]string)
	for prop, name := range t.Tags {
		if _, value, e := t.GetProperty(v, prop); e == nil {
			items[name] = value
		} else {
			err = e
			return
		}
	}

	tags = items
	return
}

// valueToString convert reflect.Value to string representation.
func (t *SentryTaggedStruct) valueToString(field reflect.Value) string {
	k := field.Kind()
	switch {
	case k == reflect.Bool:
		return strconv.FormatBool(field.Bool())
	case k >= reflect.Int && k <= reflect.Int64:
		return strconv.FormatInt(field.Int(), 10)
	case k >= reflect.Uint && k <= reflect.Uintptr:
		return strconv.FormatUint(field.Uint(), 10)
	case k == reflect.Float32 || k == reflect.Float64:
		bitSize := 32
		if k == reflect.Float64 {
			bitSize = 64
		}
		return strconv.FormatFloat(field.Float(), 'f', 12, bitSize)
	default:
		return field.String()
	}
}

// GetTags is useless for SentryTaggedScalar.
func (t *SentryTaggedScalar) GetTags() SentryTags {
	return SentryTags{}
}

// GetContextKey is getter for GinContextKey.
func (t *SentryTaggedScalar) GetContextKey() string {
	return t.GinContextKey
}

// GetName is getter for Name (tag name for scalar).
func (t *SentryTaggedScalar) GetName() string {
	return t.Name
}

// Get will extract property string representation from specified object. It will be properly formatted.
func (t *SentryTaggedScalar) Get(v interface{}) (value string, err error) {
	val := reflect.Indirect(reflect.ValueOf(v))
	if !val.IsValid() {
		err = errors.New("invalid value provided")
		return
	}

	if val.Kind() == reflect.Struct {
		err = errors.New("passed value must not be struct")
		return
	}

	if val.Type().Name() != t.Type.Name() {
		err = fmt.Errorf("passed value should be of type `%s`, got `%s` instead", t.Type.String(), val.Type().String())
		return
	}

	value = t.valueToString(val)
	return
}

// BuildTags returns map with single item in this format: <tag name> => <scalar value>.
func (t *SentryTaggedScalar) BuildTags(v interface{}) (items map[string]string, err error) {
	items = make(map[string]string)
	if value, e := t.Get(v); e == nil {
		items[t.Name] = value
	} else {
		err = e
	}
	return
}

// timeFormat is a time format helper, borrowed from gin without any changes.
func timeFormat(t time.Time) string {
	return t.Format("2006/01/02 - 15:04:05")
}
