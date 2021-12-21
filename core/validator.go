package core

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// init here will register `validateCrmURL` function for gin validator.
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("validateCrmURL", validateCrmURL); err != nil {
			panic("cannot register crm url validator: " + err.Error())
		}
	}
}

// validateCrmURL will validate CRM URL.
func validateCrmURL(fl validator.FieldLevel) bool {
	domainName := fl.Field().String()

	return isDomainValid(domainName)
}

func isDomainValid(crmURL string) bool {
	parseURL, err := url.ParseRequestURI(crmURL)

	if err != nil || nil == parseURL || !checkURLString(parseURL) {
		return false
	}

	mainDomain := getMainDomain(parseURL.Hostname())

	if checkDomains(GetSaasDomains(), mainDomain) {
		return true
	}

	if checkDomains(GetBoxDomains(), parseURL.Hostname()) {
		return true
	}

	return false
}

func checkDomains(crmDomains []Domain, domain string) bool {
	if nil == crmDomains {
		return false
	}

	for _, crmDomain := range crmDomains {
		if crmDomain.Domain == domain {
			return true
		}
	}

	return false
}

func getMainDomain(hostname string) (mainDomain string) {
	return strings.Join(strings.Split(hostname, ".")[1:], ".")
}

func checkURLString(parseURL *url.URL) bool {
	if nil == parseURL {
		return false
	}

	if parseURL.Scheme != "https" ||
		parseURL.Port() != "" ||
		(parseURL.Path != "/" && parseURL.Path != "") ||
		len(parseURL.Query()) != 0 ||
		parseURL.Fragment != "" ||
		nil != parseURL.User {
		return false
	}

	return true
}
