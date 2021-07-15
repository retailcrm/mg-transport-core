package core

import (
	"encoding/json"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// init here will register `validateCrmUrl` function for gin validator.
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("validateCrmUrl", validateCrmUrl); err != nil {
			panic("cannot register crm url validator: " + err.Error())
		}
	}
}

// validateCrmURL will validate CRM URL.
func validateCrmUrl(fl validator.FieldLevel) bool {
	domainName := fl.Field().String()
	result := isDomainValid(domainName)

	return result
}

func isDomainValid(crmUrl string) bool {
	var parseUrl = getParseUrl(crmUrl)

	if nil == parseUrl {
		return false
	}

	if !checkUrlString(parseUrl) {
		return false
	}

	var crmDomains = getValidDomains(parseUrl.Hostname())

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

func getValidDomains(hostName string) []Domain {
	subdomain := strings.Split(hostName, ".")[0]

	crmDomains := getDomainsByStore("https://infra-data.retailcrm.tech/crm-domains.json", &http.Client{})

	if nil != crmDomains {
		crmDomains = addSubdomain(subdomain, crmDomains)
	}

	boxDomains := getDomainsByStore("https://infra-data.retailcrm.tech/box-domains.json", &http.Client{})

	return append(crmDomains[:], boxDomains[:]...)
}

func addSubdomain(subdomain string, domains []Domain) []Domain {
	for key, domain := range domains {
		domains[key].Domain = subdomain + "." + domain.Domain
	}

	return domains
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
