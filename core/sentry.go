package core

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"

	"github.com/pkg/errors"
	"github.com/retailcrm/mg-transport-core/core/stacktrace"

	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
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
	Logger       LoggerInterface
	Client       stacktrace.RavenClientInterface
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
func NewSentry(sentryDSN string, defaultError string, taggedTypes SentryTaggedTypes, logger LoggerInterface, localizer *Localizer) *Sentry {
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
			stack := raven.NewStacktrace(4, 3, nil)
			recStr := fmt.Sprint(recovery)
			err := errors.New(recStr)
			go s.Client.CaptureMessageAndWait(
				recStr,
				tags,
				raven.NewException(err, stack),
				raven.NewHttp(c.Request),
			)
		}

		for _, err := range c.Errors {
			if s.Stacktrace {
				stackBuilder := stacktrace.GetStackBuilderByErrorType(err.Err)
				stackBuilder.SetClient(s.Client)
				stack, buildErr := stackBuilder.Build().GetResult()
				if buildErr != nil {
					go s.Client.CaptureErrorAndWait(buildErr, tags)
					stack = stacktrace.GenericStack(s.Client)
				}

				go s.Client.CaptureMessageAndWait(
					err.Error(),
					tags,
					raven.NewException(err.Err, stack),
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
