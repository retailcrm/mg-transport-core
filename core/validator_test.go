package core

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/retailcrm/mg-transport-core/core/db/models"
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
	crmDomains := []string{
		"https://asd.retailcrm.ru:90",
		"https://test.retailcrm.pro/test",
		"http://raisa.retailcrm.es",
		"https://blabla.simla.com#test",
		"https://test:test@blabla.simlachat.com",
	}

	for _, domain := range crmDomains {
		conn := models.Connection{
			Key: "key",
			URL: domain,
		}

		err := s.engine.Struct(conn)
		require.IsType(s.T(), validator.ValidationErrors{}, err)

		assert.Equal(
			s.T(),
			"Key: 'Connection.URL' Error:Field validation for 'URL' failed on the 'validateCrmURL' tag",
			s.getError(err))
	}
}

func (s *ValidatorSuite) Test_ValidationSuccess() {
	crmDomains := []string{
		"https://asd.retailcrm.ru",
		"https://test.retailcrm.pro",
		"https://raisa.retailcrm.es",
		"https://blabla.simla.com",
		"https://blabla.simlachat.com",
		"https://blabla.simlachat.ru",
		"https://blabla.ecomlogic.com",
		"https://crm.baucenter.ru",
		"https://crm.holodilnik.ru",
		"https://crm.eco.lanit.ru",
		"https://ecom.inventive.ru",
		"https://retailcrm.tvoydom.ru",
	}

	for _, domain := range crmDomains {
		conn := models.Connection{
			Key: "key",
			URL: domain,
		}

		err := s.engine.Struct(conn)
		assert.NoError(s.T(), err, s.getError(err))
	}
}
