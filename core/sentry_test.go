package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SampleStruct struct {
	ID      int
	Pointer *int
	Field   string
}

type SentryTest struct {
	suite.Suite
	sentry     *Sentry
	structTags *SentryTaggedStruct
	scalarTags *SentryTaggedScalar
}

func (s *SentryTest) SetupSuite() {
	s.structTags = NewTaggedStruct(SampleStruct{}, "struct", map[string]string{"fake": "prop"})
	s.scalarTags = NewTaggedScalar("", "scalar", "Scalar")
	require.Equal(s.T(), "struct", s.structTags.GetContextKey())
	require.Equal(s.T(), "scalar", s.scalarTags.GetContextKey())
	require.Equal(s.T(), "", s.structTags.GetName())
	require.Equal(s.T(), "Scalar", s.scalarTags.GetName())
	s.structTags.Tags = map[string]string{}
	s.sentry = NewSentry("dsn", "unknown_error", SentryTaggedTypes{}, nil, nil)
}

func (s *SentryTest) TestStruct_AddTag() {
	s.structTags.AddTag("test field", "Field")
	require.NotEmpty(s.T(), s.structTags.GetTags())

	tags, err := s.structTags.BuildTags(SampleStruct{Field: "value"})
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), tags)

	i, ok := tags["test field"]
	require.True(s.T(), ok)
	assert.Equal(s.T(), "value", i)
}

func (s *SentryTest) TestStruct_GetProperty() {
	s.structTags.AddTag("test field", "Field")
	name, value, err := s.structTags.GetProperty(SampleStruct{Field: "test"}, "Field")
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
	assert.Equal(s.T(), "passed value should be of type `core.SampleStruct`, got `core.Sentry` instead", err.Error())
}

func (s *SentryTest) TestStruct_GetProperty_CannotFindProperty() {
	_, _, err := s.structTags.GetProperty(SampleStruct{ID: 1}, "ID")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "cannot find property `ID`", err.Error())
}

func (s *SentryTest) TestStruct_GetProperty_InvalidProperty() {
	s.structTags.AddTag("test invalid", "Pointer")
	_, _, err := s.structTags.GetProperty(SampleStruct{Pointer: nil}, "Pointer")
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
	tags, err := s.structTags.BuildTags(SampleStruct{Field: "value"})

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
	assert.NotNil(s.T(), s.sentry.ErrorMiddleware())
}

func (s *SentryTest) TestSentry_PanicLogger() {
	assert.NotNil(s.T(), s.sentry.PanicLogger())
}

func (s *SentryTest) TestSentry_ErrorLogger() {
	assert.NotNil(s.T(), s.sentry.ErrorLogger())
}

func (s *SentryTest) TestSentry_ErrorResponseHandler() {
	assert.NotNil(s.T(), s.sentry.ErrorResponseHandler())
}

func (s *SentryTest) TestSentry_ErrorCaptureHandler() {
	assert.NotNil(s.T(), s.sentry.ErrorCaptureHandler())
}

func TestSentry_Suite(t *testing.T) {
	suite.Run(t, new(SentryTest))
}
