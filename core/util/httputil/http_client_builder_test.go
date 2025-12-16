package httputil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/retailcrm/mg-transport-core/v2/core/config"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

type HTTPClientBuilderTest struct {
	suite.Suite
	builder *HTTPClientBuilder
}

func (t *HTTPClientBuilderTest) SetupSuite() {
	t.builder = NewHTTPClientBuilder()
}

func (t *HTTPClientBuilderTest) Test_SetTimeout() {
	t.builder.SetTimeout(90)

	t.Equal(90*time.Second, t.builder.timeout)
	t.Equal(90*time.Second, t.builder.httpClient.Timeout)
}

func (t *HTTPClientBuilderTest) Test_SetLogging() {
	t.builder.SetLogging(true)
	t.True(t.builder.logging)

	t.builder.SetLogging(false)
	t.False(t.builder.logging)
}

func (t *HTTPClientBuilderTest) Test_SetSSLVerification() {
	t.builder.SetSSLVerification(true)
	t.False(t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)

	t.builder.SetSSLVerification(false)
	t.True(t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)
}

func (t *HTTPClientBuilderTest) Test_SetCertPool() {
	t.builder.SetCertPool(nil)
	t.Nil(t.builder.httpTransport.TLSClientConfig.RootCAs)

	pool := x509.NewCertPool()
	t.builder.SetCertPool(pool)
	t.Equal(pool, t.builder.httpTransport.TLSClientConfig.RootCAs)
}

func (t *HTTPClientBuilderTest) Test_SetProxy() {
	t.builder.SetProxy(nil)
	t.Nil(t.builder.proxyFunc)
	t.builder.SetProxy(http.ProxyFromEnvironment)
	t.NotNil(t.builder.proxyFunc)
}

func (t *HTTPClientBuilderTest) Test_FromConfigNil() {
	defer func() {
		t.Nil(recover())
	}()
	t.builder.FromConfig(nil)
}

func (t *HTTPClientBuilderTest) Test_FromConfig() {
	ip, err := net.DefaultResolver.LookupIPAddr(context.Background(), "simla.com")
	t.Require().NoError(err)

	vFalse := false
	config := &config.HTTPClientConfig{
		SSLVerification: boolPtr(true),
		Proxy: &config.HTTPClientProxyConfig{
			FromEnv: &vFalse,
			SplitTunnel: []config.HTTPClientProxyItem{
				{
					Proxy: "socks5://127.0.0.1:1080",
					Hosts: []string{
						"example.com",
						"google.com",
					},
					IPSet: (func() []string {
						var ips []string
						for _, i := range ip {
							ips = append(ips, i.IP.String())
						}
						return ips
					})(),
				},
			},
		},
		Timeout: 60,
	}

	client, err := t.builder.FromConfig(config).Build()
	t.Require().NoError(err)
	t.NotNil(client)
	t.Equal(!config.IsSSLVerificationEnabled(), t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)
	t.Equal(config.Timeout*time.Second, t.builder.timeout)
	t.Equal(config.Timeout*time.Second, t.builder.httpClient.Timeout)
	t.NotNil(t.builder.httpTransport.Proxy)

	getProxy := func(uri string) *url.URL {
		req, err := http.NewRequest(http.MethodGet, uri, nil)
		t.Require().NoError(err)

		proxyURL, err := t.builder.httpTransport.Proxy(req)
		t.Require().NoError(err)

		return proxyURL
	}

	t.Nil(getProxy("https://ifconfig.me"))
	t.NotNil(getProxy("https://example.com"))
	t.NotNil(getProxy("https://google.com"))
	t.Nil(getProxy("https://google.co.uk"))
	t.NotNil(getProxy("https://simla.com"))
}

func (t *HTTPClientBuilderTest) Test_buildDialer() {
	t.builder.buildDialer()

	t.NotNil(t.builder.dialer)
}

func (t *HTTPClientBuilderTest) Test_WithLogger() {
	builder := NewHTTPClientBuilder()
	require.Nil(t.T(), builder.logger)

	builder.WithLogger(nil)
	t.Nil(builder.logger)

	log := logger.NewDefault("json", true)
	builder.WithLogger(log)
	t.NotNil(builder.logger)
}

func (t *HTTPClientBuilderTest) Test_logf() {
	defer func() {
		t.Nil(recover())
	}()

	t.builder.log(fmt.Sprintf("test %s", "string"))
}

func (t *HTTPClientBuilderTest) Test_Build() {
	timeout := time.Duration(10)
	pool := x509.NewCertPool()
	client, err := t.builder.
		SetTimeout(timeout).
		SetProxy(nil).
		SetCertPool(pool).
		Build(true)

	t.Require().NoError(err)
	t.NotNil(client)
	t.Nil(client.Transport.(*http.Transport).Proxy)
	t.Equal(client, http.DefaultClient)
	t.Equal(timeout*time.Second, client.Timeout)
	t.Equal(pool, client.Transport.(*http.Transport).TLSClientConfig.RootCAs)
}

func (t *HTTPClientBuilderTest) Test_RestoreDefault() {
	t.builder.ReplaceDefault()
	t.builder.RestoreDefault()

	t.Equal(http.DefaultClient, DefaultClient)
	t.Equal(http.DefaultTransport, DefaultTransport)
}

func (t *HTTPClientBuilderTest) Test_UseTLS10() {
	client, err := NewHTTPClientBuilder().SetSSLVerification(true).UseTLS10().Build()

	t.Require().NoError(err)
	t.Require().NotNil(client)
	t.Require().NotNil(client.Transport)
	t.Require().NotNil(client.Transport.(*http.Transport).TLSClientConfig)
	t.Equal(uint16(tls.VersionTLS10), client.Transport.(*http.Transport).TLSClientConfig.MinVersion)
	t.NotNil(client.Transport.(*http.Transport).Proxy)
}

// taken from https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func Test_HTTPClientBuilder(t *testing.T) {
	suite.Run(t, new(HTTPClientBuilderTest))
}

func TestParseSubnet(t *testing.T) {
	perform := func(t *testing.T, val string) string {
		sn, err := ParseSubnet(val)
		require.NoError(t, err)
		return sn.String()
	}

	t.Run("4", func(t *testing.T) {
		items := []string{
			"192.168.1.0/24",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.1/32",
			"249.170.156.152",
			"0.0.0.0/0",
		}

		for _, item := range items {
			expected := item
			if !strings.Contains(expected, "/") {
				expected += "/32"
			}
			assert.Equal(t, expected, perform(t, item))
		}
	})

	t.Run("6", func(t *testing.T) {
		items := []string{
			"2001:db8:85a3::/64",
			"2001:0db8:0000:0042:0000:2558:0000:0001/128",
			"fe80::/64",
			"::1/128",
			"::/0",
			"2607:f8b0:4005:805::/64",
			"61b:d75b:6f6b:79ce:2b81:5a4a:9af4:9f42",
		}

		for n, item := range items {
			expected := item
			if n == 1 {
				expected = "2001:db8:0:42:0:2558:0:1/128"
			}
			if !strings.Contains(expected, "/") {
				expected += "/128"
			}
			assert.Equal(t, expected, perform(t, item))
		}
	})
}

func boolPtr(val bool) *bool {
	b := val
	return &b
}
