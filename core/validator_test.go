package core

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ValidatorSuite struct {
	suite.Suite
	engine *validator.Validate
}

func Test_Validator(t *testing.T) {
	suite.Run(t, new(ValidatorSuite))
}

func (s *ValidatorSuite) SetupSuite() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		s.engine = v
	} else {
		s.T().Fatalf("cannot obtain validation engine: %#v", v)
	}
}

func (s *ValidatorSuite) getError(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func (s *ValidatorSuite) Test_ValidationInvalidType() {
	assert.IsType(s.T(), &validator.InvalidValidationError{}, s.engine.Struct(nil))
}

func (s *ValidatorSuite) Test_ValidationFails() {
	conn := Connection{
		Key: "key",
		URL: "url",
	}
	err := s.engine.Struct(conn)
	require.IsType(s.T(), validator.ValidationErrors{}, err)
	validatorErrors := err.(validator.ValidationErrors)
	assert.Equal(
		s.T(),
		"Key: 'Connection.URL' Error:Field validation for 'URL' failed on the 'validatecrmurl' tag",
		validatorErrors.Error())
}

func (s *ValidatorSuite) Test_ValidationSuccess() {
	conn := Connection{
		Key: "key",
		URL: "https://test.retailcrm.pro",
	}
	err := s.engine.Struct(conn)
	assert.NoError(s.T(), err, s.getError(err))
}
