package core

import (
	"errors"
	"testing"

	"github.com/getsentry/raven-go"
	pkgErrors "github.com/pkg/errors"
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

func (s *SentryTest) SetupTest() {
	s.structTags = NewTaggedStruct(SampleStruct{}, "struct", map[string]string{"fake": "prop"})
	s.scalarTags = NewTaggedScalar("", "scalar", "Scalar")
	require.Equal(s.T(), "struct", s.structTags.GetContextKey())
	require.Equal(s.T(), "scalar", s.scalarTags.GetContextKey())
	require.Equal(s.T(), "", s.structTags.GetName())
	require.Equal(s.T(), "Scalar", s.scalarTags.GetName())
	s.structTags.Tags = map[string]string{}
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
	_, _, err := s.structTags.GetProperty("str", "Field")
	require.Error(s.T(), err)
	assert.Equal(s.T(), "passed value must be struct, str provided", err.Error())
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

func TestSentry_newRavenStackTrace_Fail(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	newRavenStackTrace(nil, errors.New("error"), 0)
}

func TestSentry_newRavenStackTrace(t *testing.T) {
	st := newRavenStackTrace(&raven.Client{}, errors.New("error"), 0)

	require.NotNil(t, st)
	assert.NotEmpty(t, st.Frames)
}

func TestSentry_newRavenStackTrace_ErrorsPkg(t *testing.T) {
	err := pkgErrors.New("error")
	st := newRavenStackTrace(&raven.Client{}, err, 0)

	require.NotNil(t, st)
	assert.NotEmpty(t, st.Frames)
}

func TestSentry_Suite(t *testing.T) {
	suite.Run(t, new(SentryTest))
}
