package core

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/suite"

	"github.com/retailcrm/mg-transport-core/v2/core/db/models"
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

func (s *ValidatorSuite) SetupTest() {
	crmDomainStore.update()
	boxDomainStore.update()
}

func (s *ValidatorSuite) getError(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func (s *ValidatorSuite) Test_ValidationInvalidType() {
	s.IsType(&validator.InvalidValidationError{}, s.engine.Struct(nil))
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
		s.Require().IsType(validator.ValidationErrors{}, err)

		s.Equal(
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
	}

	for _, domain := range boxDomainStore.domains {
		crmDomains = append(crmDomains, "https://"+domain.Domain)
	}

	s.Greater(len(crmDomains), 4, "No box domains were tested, test is incomplete!")

	for _, domain := range crmDomains {
		conn := models.Connection{
			Key: "key",
			URL: domain,
		}

		err := s.engine.Struct(conn)
		s.NoError(err, domain+": "+s.getError(err))
	}
}
