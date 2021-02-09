package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// DefaultClient stores original http.DefaultClient.
var DefaultClient = http.DefaultClient

// DefaultTransport stores original http.DefaultTransport.
var DefaultTransport = http.DefaultTransport

// HTTPClientBuilder builds http client with mocks (if necessary) and timeout.
// Example:
// 		// Build HTTP client with timeout = 10 sec, without SSL certificates verification and with mocked google.com
// 		client, err := NewHTTPClientBuilder().
// 			SetTimeout(10).
// 			SetMockAddress("api_mock:3004").
// 			AddMockedDomain("google.com").
// 			SetSSLVerification(false).
// 			Build()
//
// 		if err != nil {
// 			fmt.Print(err)
// 		}
//
// 		// Actual response will be returned from "api_mock:3004" (it should provide any ssl certificate)
// 		if resp, err := client.Get("https://google.com"); err == nil {
// 			if data, err := ioutil.ReadAll(resp.Body); err == nil {
// 				fmt.Printf("Data: %s", string(data))
// 			} else {
// 				fmt.Print(err)
// 			}
// 		} else {
// 			fmt.Print(err)
// 		}
type HTTPClientBuilder struct {
	httpClient    *http.Client
	httpTransport *http.Transport
	certsPool     *x509.CertPool
	dialer        *net.Dialer
	logger        LoggerInterface
	built         bool
	logging       bool
	timeout       time.Duration
	mockAddress   string
	mockHost      string
	mockPort      string
	mockedDomains []string
}

// NewHTTPClientBuilder returns HTTPClientBuilder with default values.
func NewHTTPClientBuilder() *HTTPClientBuilder {
	return &HTTPClientBuilder{
		built:         false,
		httpClient:    &http.Client{},
		httpTransport: &http.Transport{},
		timeout:       30 * time.Second,
		mockAddress:   "",
		mockedDomains: []string{},
		logging:       false,
	}
}

// WithLogger sets provided logger into HTTPClientBuilder.
func (b *HTTPClientBuilder) WithLogger(logger LoggerInterface) *HTTPClientBuilder {
	if logger != nil {
		b.logger = logger
	}

	return b
}

// SetTimeout sets timeout for http client.
func (b *HTTPClientBuilder) SetTimeout(seconds time.Duration) *HTTPClientBuilder {
	seconds = seconds * time.Second
	b.timeout = seconds
	b.httpClient.Timeout = seconds
	return b
}

// SetMockAddress sets mock address.
func (b *HTTPClientBuilder) SetMockAddress(address string) *HTTPClientBuilder {
	b.mockAddress = address
	return b
}

// AddMockedDomain adds new mocked domain.
func (b *HTTPClientBuilder) AddMockedDomain(domain string) *HTTPClientBuilder {
	b.mockedDomains = append(b.mockedDomains, domain)
	return b
}

// SetMockedDomains sets mocked domains from slice.
func (b *HTTPClientBuilder) SetMockedDomains(domains []string) *HTTPClientBuilder {
	b.mockedDomains = domains
	return b
}

// SetSSLVerification enables or disables SSL certificates verification in client.
func (b *HTTPClientBuilder) SetSSLVerification(enabled bool) *HTTPClientBuilder {
	if b.httpTransport.TLSClientConfig == nil {
		b.httpTransport.TLSClientConfig = &tls.Config{}
	}

	b.httpTransport.TLSClientConfig.InsecureSkipVerify = !enabled

	return b
}

// SetSSLVerification enables or disables SSL certificates verification in client.
func (b *HTTPClientBuilder) SetCertPool(pool *x509.CertPool) *HTTPClientBuilder {
	if b.httpTransport.TLSClientConfig == nil {
		b.httpTransport.TLSClientConfig = &tls.Config{}
	}

	b.httpTransport.TLSClientConfig.RootCAs = pool

	return b
}

// SetLogging enables or disables logging in mocks.
func (b *HTTPClientBuilder) SetLogging(flag bool) *HTTPClientBuilder {
	b.logging = flag
	return b
}

// FromConfig fulfills mock configuration from HTTPClientConfig.
func (b *HTTPClientBuilder) FromConfig(config *HTTPClientConfig) *HTTPClientBuilder {
	if config == nil {
		return b
	}

	if config.MockAddress != "" {
		b.SetMockAddress(config.MockAddress)
		b.SetMockedDomains(config.MockedDomains)
	}

	if config.Timeout > 0 {
		b.SetTimeout(config.Timeout)
	}

	b.SetSSLVerification(config.IsSSLVerificationEnabled())

	return b
}

// FromEngine fulfills mock configuration from ConfigInterface inside Engine.
func (b *HTTPClientBuilder) FromEngine(engine *Engine) *HTTPClientBuilder {
	return b.FromConfig(engine.GetHTTPClientConfig())
}

// buildDialer initializes dialer with provided timeout.
func (b *HTTPClientBuilder) buildDialer() *HTTPClientBuilder {
	b.dialer = &net.Dialer{
		Timeout:   b.timeout,
		KeepAlive: b.timeout,
	}

	return b
}

// parseAddress parses address and returns error in case of error (port is necessary).
func (b *HTTPClientBuilder) parseAddress() error {
	if b.mockAddress == "" {
		return nil
	}

	if host, port, err := net.SplitHostPort(b.mockAddress); err == nil {
		b.mockHost = host
		b.mockPort = port
	} else {
		return errors.Errorf("cannot split host and port: %s", err.Error())
	}

	return nil
}

// buildMocks builds mocks for http client.
func (b *HTTPClientBuilder) buildMocks() error {
	if b.dialer == nil {
		return errors.New("dialer must be built first")
	}

	if b.mockHost != "" && b.mockPort != "" && len(b.mockedDomains) > 0 {
		b.logf("Mock address is \"%s\"\n", net.JoinHostPort(b.mockHost, b.mockPort))
		b.logf("Mocked domains: ")

		for _, domain := range b.mockedDomains {
			b.logf(" - %s\n", domain)
		}

		b.httpTransport.DialContext = func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
			var (
				host string
				port string
				err  error
			)
			if host, port, err = net.SplitHostPort(addr); err != nil {
				return b.dialer.DialContext(ctx, network, addr)
			}

			for _, mock := range b.mockedDomains {
				if mock == host {
					oldAddr := addr

					if b.mockPort == "0" {
						addr = net.JoinHostPort(b.mockHost, port)
					} else {
						addr = net.JoinHostPort(b.mockHost, b.mockPort)
					}

					b.logf("Mocking \"%s\" with \"%s\"\n", oldAddr, addr)
				}
			}

			return b.dialer.DialContext(ctx, network, addr)
		}
	}

	return nil
}

// logf prints logs via Engine or via fmt.Printf.
func (b *HTTPClientBuilder) logf(format string, args ...interface{}) {
	if b.logging {
		if b.logger != nil {
			b.logger.Infof(format, args...)
		} else {
			fmt.Printf(format, args...)
		}
	}
}

// ReplaceDefault replaces default client and transport with generated ones.
func (b *HTTPClientBuilder) ReplaceDefault() *HTTPClientBuilder {
	if b.built {
		http.DefaultClient = b.httpClient
		http.DefaultTransport = b.httpTransport
	}

	return b
}

// RestoreDefault restores default client and transport after replacement.
func (b *HTTPClientBuilder) RestoreDefault() *HTTPClientBuilder {
	http.DefaultClient = DefaultClient
	http.DefaultTransport = DefaultTransport

	return b
}

// Build builds client, pass true to replace http.DefaultClient with generated one.
func (b *HTTPClientBuilder) Build(replaceDefault ...bool) (*http.Client, error) {
	if err := b.buildDialer().parseAddress(); err != nil {
		return nil, err
	}

	if err := b.buildMocks(); err != nil {
		return nil, err
	}

	b.built = true
	b.httpClient.Transport = b.httpTransport

	if len(replaceDefault) > 0 && replaceDefault[0] {
		b.ReplaceDefault()
	}

	return b.httpClient, nil
}
