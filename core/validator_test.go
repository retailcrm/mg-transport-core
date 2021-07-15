package core

import (
	"encoding/json"
	"github.com/h2non/gock"
	"net/http"
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
	initValidator([]string{"https://example.com/domains.json"})

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
	crmDomains := CrmDomains{
		"",
		[]Domain{{"asd.retailcrm.ru"},
			{"test.retailcrm.pro"},
			{"raisa.retailcrm.es"},
			{"blabla.simla.com"},
			{"blabla.simla.ru"},
			{"blabla.simlachat.com"},
			{"blabla.simlachat.ru"},
		},
	}

	data, _ := json.Marshal(crmDomains)

	for _, domain:= range crmDomains.Domains {
		gock.New("https://example.com").
			Get("/domains.json").
			Reply(http.StatusOK).
			BodyString(string(data))

		conn := Connection{
			Key: "key",
			URL: "https://" + domain.Domain,
		}
		err := s.engine.Struct(conn)
		assert.NoError(s.T(), err, s.getError(err))

		gock.Off()
	}
}
