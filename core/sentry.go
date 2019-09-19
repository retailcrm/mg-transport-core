package core

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/pkg/errors"

	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

// ErrorHandlerFunc will handle errors
type ErrorHandlerFunc func(recovery interface{}, c *gin.Context)

// SentryTaggedTypes list
type SentryTaggedTypes []SentryTagged

// SentryTags list for SentryTaggedStruct. Format: name => property name
type SentryTags map[string]string

// SentryTagged interface for both tagged scalar and struct
type SentryTagged interface {
	BuildTags(interface{}) (map[string]string, error)
	GetContextKey() string
	GetTags() SentryTags
	GetName() string
}

// Sentry struct. Holds SentryTaggedStruct list
type Sentry struct {
	TaggedTypes  SentryTaggedTypes
	Stacktrace   bool
	DefaultError string
	Localizer    *Localizer
	Logger       *logging.Logger
	Client       *raven.Client
}

// SentryTaggedStruct holds information about type, it's key in gin.Context (for middleware), and it's properties
type SentryTaggedStruct struct {
	Type          reflect.Type
	GinContextKey string
	Tags          SentryTags
}

// SentryTaggedScalar variable from context
type SentryTaggedScalar struct {
	SentryTaggedStruct
	Name string
}

// NewSentry constructor
func NewSentry(sentryDSN string, defaultError string, taggedTypes SentryTaggedTypes, logger *logging.Logger, localizer *Localizer) *Sentry {
	sentry := &Sentry{
		DefaultError: defaultError,
		TaggedTypes:  taggedTypes,
		Localizer:    localizer,
		Logger:       logger,
		Stacktrace:   true,
	}
	sentry.createRavenClient(sentryDSN)
	return sentry
}

// NewTaggedStruct constructor
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

// NewTaggedScalar constructor
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

// createRavenClient will init raven.Client
func (s *Sentry) createRavenClient(sentryDSN string) {
	client, _ := raven.New(sentryDSN)
	s.Client = client
}

// combineGinErrorHandlers calls several error handlers simultaneously
func (s *Sentry) combineGinErrorHandlers(handlers ...ErrorHandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			for _, handler := range handlers {
				handler(rec, c)
			}

			if rec != nil || len(c.Errors) > 0 {
				c.Abort()
			}
		}()

		c.Next()
	}
}

// ErrorMiddleware returns error handlers, attachable to gin.Engine
func (s *Sentry) ErrorMiddleware() gin.HandlerFunc {
	defaultHandlers := []ErrorHandlerFunc{
		s.ErrorResponseHandler(),
		s.PanicLogger(),
		s.ErrorLogger(),
	}

	if s.Client != nil {
		defaultHandlers = append(defaultHandlers, s.ErrorCaptureHandler())
	}

	return s.combineGinErrorHandlers(defaultHandlers...)
}

// PanicLogger logs panic
func (s *Sentry) PanicLogger() ErrorHandlerFunc {
	return func(recovery interface{}, c *gin.Context) {
		if recovery != nil {
			if s.Logger != nil {
				s.Logger.Error(c.Request.RequestURI, recovery)
			} else {
				fmt.Print("ERROR =>", c.Request.RequestURI, recovery)
			}
			debug.PrintStack()
		}
	}
}

// ErrorLogger logs basic errors
func (s *Sentry) ErrorLogger() ErrorHandlerFunc {
	return func(recovery interface{}, c *gin.Context) {
		for _, err := range c.Errors {
			if s.Logger != nil {
				s.Logger.Error(c.Request.RequestURI, err.Err)
			} else {
				fmt.Print("ERROR =>", c.Request.RequestURI, err.Err)
			}
		}
	}
}

// ErrorResponseHandler will be executed in case of any unexpected error
func (s *Sentry) ErrorResponseHandler() ErrorHandlerFunc {
	return func(recovery interface{}, c *gin.Context) {
		publicErrors := c.Errors.ByType(gin.ErrorTypePublic)
		privateLen := len(c.Errors.ByType(gin.ErrorTypePrivate))
		publicLen := len(publicErrors)

		if privateLen == 0 && publicLen == 0 && recovery == nil {
			return
		}

		messagesLen := publicLen
		if privateLen > 0 || recovery != nil {
			messagesLen++
		}

		messages := make([]string, messagesLen)
		index := 0
		for _, err := range publicErrors {
			messages[index] = err.Error()
			index++
		}

		if privateLen > 0 || recovery != nil {
			if s.Localizer == nil {
				messages[index] = s.DefaultError
			} else {
				messages[index] = s.Localizer.GetLocalizedMessage(s.DefaultError)
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": messages})
	}
}

// ErrorCaptureHandler will generate error data and send it to sentry
func (s *Sentry) ErrorCaptureHandler() ErrorHandlerFunc {
	return func(recovery interface{}, c *gin.Context) {
		tags := map[string]string{
			"endpoint": c.Request.RequestURI,
		}

		if len(s.TaggedTypes) > 0 {
			for _, tagged := range s.TaggedTypes {
				if item, ok := c.Get(tagged.GetContextKey()); ok && item != nil {
					if itemTags, err := tagged.BuildTags(item); err == nil {
						for tagName, tagValue := range itemTags {
							tags[tagName] = tagValue
						}
					}
				}
			}
		}

		if recovery != nil {
			stacktrace := raven.NewStacktrace(4, 3, nil)
			recStr := fmt.Sprint(recovery)
			err := errors.New(recStr)
			go s.Client.CaptureMessageAndWait(
				recStr,
				tags,
				raven.NewException(err, stacktrace),
				raven.NewHttp(c.Request),
			)
		}

		for _, err := range c.Errors {
			if s.Stacktrace {
				stacktrace := newRavenStackTrace(s.Client, err.Err, 0)
				go s.Client.CaptureMessageAndWait(
					err.Error(),
					tags,
					raven.NewException(err.Err, stacktrace),
					raven.NewHttp(c.Request),
				)
			} else {
				go s.Client.CaptureErrorAndWait(err.Err, tags)
			}
		}
	}
}

// AddTag will add tag with property name which holds tag in object
func (t *SentryTaggedStruct) AddTag(name string, property string) *SentryTaggedStruct {
	t.Tags[property] = name
	return t
}

// GetTags is Tags getter
func (t *SentryTaggedStruct) GetTags() SentryTags {
	return t.Tags
}

// GetContextKey is GinContextKey getter
func (t *SentryTaggedStruct) GetContextKey() string {
	return t.GinContextKey
}

// GetName is useless for SentryTaggedStruct
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
		err = fmt.Errorf("passed value must be struct, %s provided", val.String())
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

// BuildTags will extract tags for Sentry from specified object
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

// valueToString convert reflect.Value to string representation
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

// GetTags is useless for SentryTaggedScalar
func (t *SentryTaggedScalar) GetTags() SentryTags {
	return SentryTags{}
}

// GetContextKey is getter for GinContextKey
func (t *SentryTaggedScalar) GetContextKey() string {
	return t.GinContextKey
}

// GetName is getter for Name (tag name for scalar)
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

// BuildTags returns map with single item in this format: <tag name> => <scalar value>
func (t *SentryTaggedScalar) BuildTags(v interface{}) (items map[string]string, err error) {
	items = make(map[string]string)
	if value, e := t.Get(v); e == nil {
		items[t.Name] = value
	} else {
		err = e
	}
	return
}

// newRavenStackTrace generate stacktrace compatible with raven-go format
// It tries to extract better stacktrace from error from package "github.com/pkg/errors"
// In case of fail it will fallback to default stacktrace generation from raven-go.
// Default stacktrace highly likely will be useless, because it will not include call
// which returned error. This occurs because default stacktrace doesn't include any call
// before stacktrace generation, and raven-go will generate stacktrace here, which will end
// in trace to this file. But errors from "github.com/pkg/errors" will generate stacktrace
// immediately, it will include call which returned error, and we can fetch this trace.
// Also we can wrap default errors with error from this package, like this:
//      errors.Wrap(err, err.Error)
func newRavenStackTrace(client *raven.Client, myerr error, skip int) *raven.Stacktrace {
	st := getErrorStackTraceConverted(myerr, 3, client.IncludePaths())
	if st == nil {
		st = raven.NewStacktrace(skip, 3, client.IncludePaths())
	}
	return st
}

// getErrorStackTraceConverted will return converted stacktrace from custom error, or nil in case of default error
func getErrorStackTraceConverted(err error, context int, appPackagePrefixes []string) *raven.Stacktrace {
	st := getErrorCauseStackTrace(err)
	if st == nil {
		return nil
	}
	return convertStackTrace(st, context, appPackagePrefixes)
}

// getErrorCauseStackTrace tries to extract stacktrace from custom error, returns nil in case of failure
func getErrorCauseStackTrace(err error) errors.StackTrace {
	// This code is inspired by github.com/pkg/errors.Cause().
	var st errors.StackTrace
	for err != nil {
		s := getErrorStackTrace(err)
		if s != nil {
			st = s
		}
		err = getErrorCause(err)
	}
	return st
}

// convertStackTrace converts github.com/pkg/errors.StackTrace to github.com/getsentry/raven-go.Stacktrace
func convertStackTrace(st errors.StackTrace, context int, appPackagePrefixes []string) *raven.Stacktrace {
	// This code is borrowed from github.com/getsentry/raven-go.NewStacktrace().
	var frames []*raven.StacktraceFrame
	for _, f := range st {
		frame := convertFrame(f, context, appPackagePrefixes)
		if frame != nil {
			frames = append(frames, frame)
		}
	}
	if len(frames) == 0 {
		return nil
	}
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}
	return &raven.Stacktrace{Frames: frames}
}

// convertFrame converts single frame from github.com/pkg/errors.Frame to github.com/pkg/errors.Frame
func convertFrame(f errors.Frame, context int, appPackagePrefixes []string) *raven.StacktraceFrame {
	// This code is borrowed from github.com/pkg/errors.Frame.
	pc := uintptr(f) - 1
	fn := runtime.FuncForPC(pc)
	var file string
	var line int
	if fn != nil {
		file, line = fn.FileLine(pc)
	} else {
		file = "unknown"
	}
	return raven.NewStacktraceFrame(pc, file, line, context, appPackagePrefixes)
}

// getErrorStackTrace will try to extract stacktrace from error using StackTrace method (default errors doesn't have it)
func getErrorStackTrace(err error) errors.StackTrace {
	ster, ok := err.(interface {
		StackTrace() errors.StackTrace
	})
	if !ok {
		return nil
	}
	return ster.StackTrace()
}

// getErrorCause will try to extract original error from wrapper - it is used only if stacktrace is not present
func getErrorCause(err error) error {
	cer, ok := err.(interface {
		Cause() error
	})
	if !ok {
		return nil
	}
	return cer.Cause()
}
