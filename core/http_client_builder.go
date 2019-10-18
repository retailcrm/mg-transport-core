package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

var (
	DefaultClient    = http.DefaultClient
	DefaultTransport = http.DefaultTransport
)

// HTTPClientBuilder builds http client with mocks (if necessary) and timeout
type HTTPClientBuilder struct {
	httpClient    *http.Client
	httpTransport *http.Transport
	dialer        *net.Dialer
	engine        *Engine
	built         bool
	logging       bool
	timeout       time.Duration
	mockAddress   string
	mockHost      string
	mockPort      string
	mockedDomains []string
}

// NewHTTPClientBuilder returns HTTPClientBuilder with default values
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

// SetTimeout sets timeout for http client
func (b *HTTPClientBuilder) SetTimeout(timeout time.Duration) *HTTPClientBuilder {
	timeout = timeout * time.Second
	b.timeout = timeout
	b.httpClient.Timeout = timeout
	return b
}

// SetMockAddress sets mock address
func (b *HTTPClientBuilder) SetMockAddress(address string) *HTTPClientBuilder {
	b.mockAddress = address
	return b
}

// AddMockedDomain adds new mocked domain
func (b *HTTPClientBuilder) AddMockedDomain(domain string) *HTTPClientBuilder {
	b.mockedDomains = append(b.mockedDomains, domain)
	return b
}

// SetMockedDomains sets mocked domains from slice
func (b *HTTPClientBuilder) SetMockedDomains(domains []string) *HTTPClientBuilder {
	b.mockedDomains = domains
	return b
}

// DisableSSLVerification disables SSL certificates verification in client
func (b *HTTPClientBuilder) DisableSSLVerification() *HTTPClientBuilder {
	b.logf("WARNING: SSL verification is now disabled, don't use this parameter in production!")

	b.httpTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	return b
}

// EnableLogging enables logging in mocks
func (b *HTTPClientBuilder) EnableLogging() *HTTPClientBuilder {
	b.logging = true
	return b
}

// FromConfig fulfills mock configuration from HTTPClientConfig
func (b *HTTPClientBuilder) FromConfig(config *HTTPClientConfig) *HTTPClientBuilder {
	if config == nil {
		return b
	}

	if config.MockAddress != "" {
		b.mockAddress = config.MockAddress
		b.mockedDomains = config.MockedDomains
	}

	if !config.SSLVerification {
		b.DisableSSLVerification()
	}

	if config.Timeout > 0 {
		b.SetTimeout(config.Timeout)
	}

	return b
}

// FromEngine fulfills mock configuration from ConfigInterface inside Engine
func (b *HTTPClientBuilder) FromEngine(engine *Engine) *HTTPClientBuilder {
	b.engine = engine
	b.logging = engine.Config.IsDebug()
	return b.FromConfig(engine.Config.GetHTTPClientConfig())
}

// buildDialer initializes dialer with provided timeout
func (b *HTTPClientBuilder) buildDialer() *HTTPClientBuilder {
	b.dialer = &net.Dialer{
		Timeout:   b.timeout,
		KeepAlive: b.timeout,
	}

	return b
}

// parseAddress parses address and returns error in case of error (port is necessary)
func (b *HTTPClientBuilder) parseAddress() error {
	if host, port, err := net.SplitHostPort(b.mockAddress); err == nil {
		b.mockHost = host
		b.mockPort = port
		return nil
	} else {
		return errors.Errorf("cannot split host and port: %s", err.Error())
	}
}

// buildMocks builds mocks for http client
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
			if host, port, err := net.SplitHostPort(addr); err != nil {
				return b.dialer.DialContext(ctx, network, addr)
			} else {
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
			}

			return b.dialer.DialContext(ctx, network, addr)
		}
	}

	return nil
}

// logf prints logs via Engine or via fmt.Printf
func (b *HTTPClientBuilder) logf(format string, args ...interface{}) {
	if b.logging {
		if b.engine != nil && b.engine.Logger != nil {
			b.engine.Logger.Infof(format, args...)
		} else {
			fmt.Printf(format, args...)
		}
	}
}

// ReplaceDefault replaces default client and transport with generated ones
func (b *HTTPClientBuilder) ReplaceDefault() *HTTPClientBuilder {
	if b.built {
		http.DefaultClient = b.httpClient
		http.DefaultTransport = b.httpTransport
	}

	return b
}

// RestoreDefault restores default client and transport after replacement
func (b *HTTPClientBuilder) RestoreDefault() *HTTPClientBuilder {
	http.DefaultClient = DefaultClient
	http.DefaultTransport = DefaultTransport

	return b
}

// Build builds client, pass true to replace http.DefaultClient with generated one
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
