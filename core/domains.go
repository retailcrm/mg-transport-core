package core

import (
	"encoding/json"
	"io"
	"net/http"
)

const crmDomainsURL string = "https://infra-data.retailcrm.tech/crm-domains.json"
const boxDomainsURL string = "https://infra-data.retailcrm.tech/box-domains.json"

type Domain struct {
	Domain string `json:"domain"`
}

type CrmDomains struct {
	CreateDate string   `json:"createDate"`
	Domains    []Domain `json:"domains"`
}

func GetSaasDomains() []Domain {
	return getDomainsByStore(crmDomainsURL)
}

func GetBoxDomains() []Domain {
	return getDomainsByStore(boxDomainsURL)
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

	respBody, readErr := io.ReadAll(resp.Body)

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
