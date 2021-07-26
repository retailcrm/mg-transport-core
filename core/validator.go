package core

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

const crmDomainsURL string = "https://infra-data.retailcrm.tech/crm-domains.json"
const boxDomainsURL string = "https://infra-data.retailcrm.tech/box-domains.json"

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

	if checkDomains(crmDomainsURL, mainDomain) {
		return true
	}

	if checkDomains(boxDomainsURL, parseURL.Hostname()) {
		return true
	}

	return false
}

func checkDomains(domainsStoreURL string, domain string) bool {
	crmDomains := getDomainsByStore(domainsStoreURL)

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
		parseURL.User.Username() != "" {
		return false
	}

	return true
}

func getDomainsByStore(store string) []Domain {
	req, reqErr := http.NewRequest(http.MethodGet, store, nil)

	if reqErr != nil {
		return nil
	}

	req.Header.Add("Accept", "application/json")
	resp, respErr := http.DefaultClient.Do(req)

	if respErr != nil {
		return nil
	}

	respBody, readErr := ioutil.ReadAll(resp.Body)

	if readErr != nil {
		return nil
	}

	var crmDomains CrmDomains

	err := json.Unmarshal(respBody, &crmDomains)

	if err != nil {
		return nil
	}

	_ = resp.Body.Close()

	return crmDomains.Domains
}

type Domain struct {
	Domain string `json:"domain"`
}

type CrmDomains struct {
	CreateDate string   `json:"createDate"`
	Domains    []Domain `json:"domains"`
}
