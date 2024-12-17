package core

import (
	"go.uber.org/atomic"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var (
	crmDomainStore = &domainStore{
		source: GetSaasDomains,
		matcher: func(domain string, domains []Domain) bool {
			if len(domains) == 0 {
				return false
			}

			secondLevel := strings.Join(strings.Split(domain, ".")[1:], ".")

			for _, crmDomain := range domains {
				if crmDomain.Domain == secondLevel {
					return true
				}
			}

			return false
		},
	}
	boxDomainStore = &domainStore{
		source: GetBoxDomains,
		matcher: func(domain string, domains []Domain) bool {
			if len(domains) == 0 {
				return false
			}

			for _, crmDomain := range domains {
				if crmDomain.Domain == domain {
					return true
				}
			}

			return false
		},
	}
)

type domainStore struct {
	domains    []Domain
	mutex      sync.RWMutex
	source     func() []Domain
	matcher    func(string, []Domain) bool
	lastUpdate atomic.Time
}

func (ds *domainStore) match(domain string) bool {
	if time.Since(ds.lastUpdate.Load()) > time.Hour {
		ds.update()
	}
	defer ds.mutex.RUnlock()
	ds.mutex.RLock()
	return ds.matcher(domain, ds.domains)
}

func (ds *domainStore) update() {
	defer ds.mutex.Unlock()
	ds.mutex.Lock()
	ds.domains = ds.source()
	ds.lastUpdate.Store(time.Now())
}

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

	hostname := parseURL.Hostname()
	if crmDomainStore.match(hostname) {
		return true
	}

	return boxDomainStore.match(hostname)
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
