package httputil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

const (
	defaultDialerTimeout         = 30 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
	defaultMaxIdleConns          = 100
)

// DefaultClient stores original http.DefaultClient.
var DefaultClient = http.DefaultClient

// DefaultTransport stores original http.DefaultTransport.
var DefaultTransport = http.DefaultTransport

// HTTPClientBuilder builds http client with mocks (if necessary) and timeout.
// Example:
//
//	// Build HTTP client with timeout = 10 sec, without SSL certificates verification and with mocked google.com
//	client, err := NewHTTPClientBuilder().
//		SetTimeout(10).
//		SetMockAddress("api_mock:3004").
//		AddMockedDomain("google.com").
//		SetSSLVerification(false).
//		Build()
//
//	if err != nil {
//		fmt.Print(err)
//	}
//
//	// Actual response will be returned from "api_mock:3004" (it should provide any ssl certificate)
//	if resp, err := client.Get("https://google.com"); err == nil {
//		if data, err := ioutil.ReadAll(resp.Body); err == nil {
//			fmt.Printf("Data: %s", string(data))
//		} else {
//			fmt.Print(err)
//		}
//	} else {
//		fmt.Print(err)
//	}
type HTTPClientBuilder struct {
	logger        logger.Logger
	httpClient    *http.Client
	httpTransport *http.Transport
	dialer        *net.Dialer
	proxyFunc     func(req *http.Request) (*url.URL, error)
	proxyHosts    []ProxiedHost
	config        *config.HTTPClientConfig
	timeout       time.Duration
	tlsVersion    uint16
	logging       bool
	built         bool
}

// ProxiedHost is a pair of proxy config & host.
type ProxiedHost struct {
	Hosts    []string
	IPSet    []*net.IPNet
	ProxyURL *url.URL
}

// NewHTTPClientBuilder returns HTTPClientBuilder with default values.
func NewHTTPClientBuilder() *HTTPClientBuilder {
	return &HTTPClientBuilder{
		built:      false,
		httpClient: &http.Client{},
		httpTransport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   defaultDialerTimeout,
				KeepAlive: defaultDialerTimeout,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          defaultMaxIdleConns,
			IdleConnTimeout:       defaultIdleConnTimeout,
			TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
			ExpectContinueTimeout: defaultExpectContinueTimeout,
		},
		proxyFunc:  http.ProxyFromEnvironment,
		tlsVersion: tls.VersionTLS12,
		timeout:    defaultDialerTimeout,
		logging:    false,
	}
}

// WithLogger sets provided logger into HTTPClientBuilder.
func (b *HTTPClientBuilder) WithLogger(logger logger.Logger) *HTTPClientBuilder {
	if logger != nil {
		b.logger = logger
	}

	return b
}

// SetTimeout sets timeout for http client.
func (b *HTTPClientBuilder) SetTimeout(seconds time.Duration) *HTTPClientBuilder {
	seconds *= time.Second
	b.timeout = seconds
	b.httpClient.Timeout = seconds
	return b
}

// SetSSLVerification enables or disables SSL certificates verification in client.
func (b *HTTPClientBuilder) SetSSLVerification(enabled bool) *HTTPClientBuilder {
	if b.httpTransport.TLSClientConfig == nil {
		b.httpTransport.TLSClientConfig = b.baseTLSConfig()
	}

	b.httpTransport.TLSClientConfig.InsecureSkipVerify = !enabled

	return b
}

// UseTLS10 restores TLS 1.0 as a minimal supported TLS version.
func (b *HTTPClientBuilder) UseTLS10() *HTTPClientBuilder {
	b.tlsVersion = tls.VersionTLS10
	if b.httpTransport.TLSClientConfig != nil {
		b.httpTransport.TLSClientConfig.MinVersion = b.tlsVersion
	}
	return b
}

// SetProxy sets proxy function to the builder.
func (b *HTTPClientBuilder) SetProxy(proxyFunc func(req *http.Request) (*url.URL, error)) *HTTPClientBuilder {
	b.proxyFunc = proxyFunc
	return b
}

// SetProxyHosts sets a list of hosts which will be proxied.
func (b *HTTPClientBuilder) SetProxyHosts(list []ProxiedHost) *HTTPClientBuilder {
	b.proxyHosts = list
	return b
}

// SetCertPool sets provided TLS certificates pool into the client.
func (b *HTTPClientBuilder) SetCertPool(pool *x509.CertPool) *HTTPClientBuilder {
	if b.httpTransport.TLSClientConfig == nil {
		b.httpTransport.TLSClientConfig = b.baseTLSConfig()
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
func (b *HTTPClientBuilder) FromConfig(config *config.HTTPClientConfig) *HTTPClientBuilder {
	if config == nil {
		return b
	}

	b.config = config

	return b
}

// baseTLSConfig returns *tls.Config with TLS 1.2 as a minimal supported version.
func (b *HTTPClientBuilder) baseTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: b.tlsVersion} // nolint:gosec
}

// buildDialer initializes dialer with provided timeout.
func (b *HTTPClientBuilder) buildDialer() *HTTPClientBuilder {
	b.dialer = &net.Dialer{
		Timeout:   b.timeout,
		KeepAlive: b.timeout,
	}

	return b
}

// log prints logs via Engine or via fmt.Println.
func (b *HTTPClientBuilder) log(msg string, args ...interface{}) {
	if b.logging {
		if b.logger != nil {
			b.logger.Info(msg, logger.AnyZapFields(args)...)
		} else {
			fmt.Println(append([]any{msg}, args...))
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

func (b *HTTPClientBuilder) buildTransportProxy() {
	if len(b.proxyHosts) == 0 {
		b.httpTransport.Proxy = b.proxyFunc
		return
	}

	proxyFunc := func(_ *http.Request) (*url.URL, error) {
		return nil, nil
	}

	if b.proxyFunc != nil {
		proxyFunc = b.proxyFunc
	}

	b.httpTransport.Proxy = func(r *http.Request) (*url.URL, error) {
		host := r.URL.Hostname()

		var ips []net.IPAddr

		for _, d := range b.proxyHosts {
			for _, h := range d.Hosts {
				if strings.EqualFold(host, h) || strings.HasSuffix(host, "."+h) {
					return d.ProxyURL, nil
				}
			}

			if len(d.IPSet) > 0 {
				if len(ips) == 0 {
					ipList, err := net.DefaultResolver.LookupIPAddr(r.Context(), host)
					if err != nil {
						return nil, err
					}

					ips = ipList
				}

				for _, ip := range ips {
					for _, cidr := range d.IPSet {
						if cidr.Contains(ip.IP) {
							return d.ProxyURL, nil
						}
					}
				}
			}
		}

		return proxyFunc(r)
	}
}

// Build builds client, pass true to replace http.DefaultClient with generated one.
func (b *HTTPClientBuilder) Build(replaceDefault ...bool) (*http.Client, error) {
	if b.config != nil { //nolint:nestif
		if b.config.Proxy != nil {
			if b.config.Proxy.FromEnv != nil && !*b.config.Proxy.FromEnv {
				b.proxyFunc = nil
			}

			if b.config.Proxy.URL != "" {
				proxyURL, err := url.Parse(b.config.Proxy.URL)
				if err != nil {
					return nil, err
				}

				b.proxyFunc = http.ProxyURL(proxyURL)
			}

			if len(b.config.Proxy.SplitTunnel) > 0 {
				proxiedHosts := make([]ProxiedHost, 0, len(b.config.Proxy.SplitTunnel))

				for _, tunnel := range b.config.Proxy.SplitTunnel {
					proxyURL, err := url.Parse(tunnel.Proxy)
					if err != nil {
						return nil, err
					}

					var ipset []*net.IPNet

					for _, val := range tunnel.IPSet {
						subnet, err := ParseSubnet(val)
						if err != nil {
							return nil, err
						}

						ipset = append(ipset, subnet)
					}

					proxiedHosts = append(proxiedHosts, ProxiedHost{
						Hosts:    tunnel.Hosts,
						IPSet:    ipset,
						ProxyURL: proxyURL,
					})
				}

				b.SetProxyHosts(proxiedHosts)
			}
		}

		if b.config.Timeout > 0 {
			b.SetTimeout(b.config.Timeout)
		}

		b.SetSSLVerification(b.config.IsSSLVerificationEnabled())
	}

	b.buildDialer()
	b.buildTransportProxy()

	b.built = true
	b.httpClient.Transport = b.httpTransport

	if len(replaceDefault) > 0 && replaceDefault[0] {
		b.ReplaceDefault()
	}

	return b.httpClient, nil
}

// ParseSubnet will parse provided string as *net.IPNet. Plain IP's will be converted into /32 or /128 subnets.
func ParseSubnet(val string) (*net.IPNet, error) {
	if !strings.Contains(val, "/") {
		a, err := netip.ParseAddr(val)
		if err != nil {
			return nil, fmt.Errorf("invalid ip %q: %w", val, err)
		}
		bits := 32
		if a.Is6() {
			bits = 128
		}
		p := netip.PrefixFrom(a, bits).Masked()
		return prefixToIPNet(p), nil
	}

	p, err := netip.ParsePrefix(val)
	if err != nil {
		return nil, fmt.Errorf("invalid cidr %q: %w", val, err)
	}
	p = p.Masked()
	return prefixToIPNet(p), nil
}

func prefixToIPNet(p netip.Prefix) *net.IPNet {
	ip := net.IP(p.Addr().AsSlice())
	ones := p.Bits()
	bits := 128
	if p.Addr().Is4() {
		bits = 32
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(ones, bits)}
}
