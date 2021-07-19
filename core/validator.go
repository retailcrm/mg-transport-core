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

const crmDomainsUrl string = "https://infra-data.retailcrm.tech/crm-domains.json"
const boxDomainsUrl string = "https://infra-data.retailcrm.tech/box-domains.json"

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
	parseUrl, err := url.ParseRequestURI(crmUrl)

	if err != nil || nil == parseUrl || !checkUrlString(parseUrl) {
		return false
	}

	mainDomain, domainDeep := getMainDomain(parseUrl.Hostname())

	if domainDeep != 3 {
		return false
	}

	if true == checkDomains(crmDomainsUrl, mainDomain){
		return true
	}

	if true == checkDomains(boxDomainsUrl, parseUrl.Hostname()){
		return true
	}

	return false
}

func checkDomains(domainsStoreUrl string, domain string) bool {
	crmDomains := getDomainsByStore(domainsStoreUrl, http.DefaultClient)

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

func getMainDomain(hostname string) (mainDomain string, domainDeep int) {
	domainArray := strings.Split(hostname, ".")
	domainDeep = len(domainArray)
	domainArray = append(domainArray[:0], domainArray[1:]...)

	return strings.Join(domainArray, "."), domainDeep
}

func checkUrlString(parseUrl *url.URL) bool {
	if parseUrl.Scheme != "https" {
		return false
	}

	if len(parseUrl.Query()) != 0 && parseUrl.Fragment == "" {
		return false
	}

	if parseUrl.Path != "/" && parseUrl.Path != "" {
		return false
	}

	return true
}

func getDomainsByStore(store string, client *http.Client) []Domain {
	req, reqErr := http.NewRequest(http.MethodGet, store, nil); if reqErr != nil {
		return nil
	}
	req.Header.Add("Accept", "application/json")
	resp, respErr := client.Do(req); if respErr != nil {
		return nil
	}

	defer func(body io.ReadCloser) {
		_ = body.Close()
	} (resp.Body)

	respBody, readErr := ioutil.ReadAll(resp.Body); if readErr != nil {
		return nil
	}

	var crmDomains CrmDomains

	err := json.Unmarshal(respBody, &crmDomains); if err != nil {
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
