package core

import (
	"encoding/json"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

var domainStores []string

// init here will register `validatecrmurl` function for gin validator.
func initValidator(DomainStores []string) {
	domainStores = DomainStores

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("validatecrmurl", validateCrmURL); err != nil {
			panic("cannot register crm url validator: " + err.Error())
		}
	}
}

// validateCrmURL will validate CRM URL.
func validateCrmURL(fl validator.FieldLevel) bool {
	return isDomainValid(fl.Field().String())
}

func isDomainValid(crmUrl string) bool {
	var parseUrl = getParseUrl(crmUrl)

	if nil == parseUrl {
		return false
	}

	if !checkUrlString(parseUrl) {
		return false
	}

	var crmDomains = getValidDomains()

	for _, domain := range crmDomains {
		if domain.Domain == parseUrl.Hostname() {
			return true
		}
	}

	return false
}

func getParseUrl(crmUrl string) *url.URL {
	parseUrl, err := url.ParseRequestURI(crmUrl)

	if err != nil {
		return nil
	}

	return parseUrl
}

func checkUrlString(parseUrl *url.URL) bool {
	if parseUrl.Scheme != "https" {
		return false
	}

	if parseUrl.Path != "/" && parseUrl.Path != "" {
		return false
	}

	return true
}

func getValidDomains() []Domain {
	var allDomains []Domain

	for _, store := range domainStores {
		storeDomains := getDomainsByStore(store, &http.Client{})
		allDomains = append(allDomains[:], storeDomains[:]...)
	}

	return allDomains
}

func getDomainsByStore(store string, client *http.Client) []Domain {
	req, _ := http.NewRequest("GET", store, nil)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		return nil
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()

		if err != nil {
			panic(err)
		}
	}(resp.Body)

	respBody, _ := ioutil.ReadAll(resp.Body)

	var crmDomains CrmDomains

	err = json.Unmarshal(respBody, &crmDomains)

	if err != nil {
		return nil
	}

	return crmDomains.Domains
}

type Domain struct {
	Domain string `json:"domain"`
}

type CrmDomains struct {
	CreateDate string `json:"createDate"`
	Domains   []Domain `json:"domains"`
}
