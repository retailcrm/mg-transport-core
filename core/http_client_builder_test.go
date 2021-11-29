package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/retailcrm/mg-transport-core/core/logger"

	"github.com/retailcrm/mg-transport-core/core/errorutil"
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

	assert.Equal(t.T(), 90*time.Second, t.builder.timeout)
	assert.Equal(t.T(), 90*time.Second, t.builder.httpClient.Timeout)
}

func (t *HTTPClientBuilderTest) Test_SetLogging() {
	t.builder.SetLogging(true)
	assert.True(t.T(), t.builder.logging)

	t.builder.SetLogging(false)
	assert.False(t.T(), t.builder.logging)
}

func (t *HTTPClientBuilderTest) Test_SetMockAddress() {
	addr := "mock.local:3004"
	t.builder.SetMockAddress(addr)

	assert.Equal(t.T(), addr, t.builder.mockAddress)
}

func (t *HTTPClientBuilderTest) Test_AddMockedDomain() {
	domain := "example.com"
	t.builder.AddMockedDomain(domain)

	assert.NotEmpty(t.T(), t.builder.mockedDomains)
	assert.Equal(t.T(), domain, t.builder.mockedDomains[0])
}

func (t *HTTPClientBuilderTest) Test_SetMockedDomains() {
	domains := []string{"example.com"}
	t.builder.SetMockedDomains(domains)

	assert.NotEmpty(t.T(), t.builder.mockedDomains)
	assert.Equal(t.T(), domains[0], t.builder.mockedDomains[0])
}

func (t *HTTPClientBuilderTest) Test_SetSSLVerification() {
	t.builder.SetSSLVerification(true)
	assert.False(t.T(), t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)

	t.builder.SetSSLVerification(false)
	assert.True(t.T(), t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)
}

func (t *HTTPClientBuilderTest) Test_SetCertPool() {
	t.builder.SetCertPool(nil)
	assert.Nil(t.T(), t.builder.httpTransport.TLSClientConfig.RootCAs)

	pool := x509.NewCertPool()
	t.builder.SetCertPool(pool)
	assert.Equal(t.T(), pool, t.builder.httpTransport.TLSClientConfig.RootCAs)
}

func (t *HTTPClientBuilderTest) Test_FromConfigNil() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()
	t.builder.FromConfig(nil)
}

func (t *HTTPClientBuilderTest) Test_FromConfig() {
	config := &HTTPClientConfig{
		SSLVerification: boolPtr(true),
		MockAddress:     "anothermock.local:3004",
		MockedDomains:   []string{"example.gov"},
		Timeout:         60,
	}

	t.builder.FromConfig(config)
	assert.Equal(t.T(), !config.IsSSLVerificationEnabled(), t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)
	assert.Equal(t.T(), config.MockAddress, t.builder.mockAddress)
	assert.Equal(t.T(), config.MockedDomains[0], t.builder.mockedDomains[0])
	assert.Equal(t.T(), config.Timeout*time.Second, t.builder.timeout)
	assert.Equal(t.T(), config.Timeout*time.Second, t.builder.httpClient.Timeout)
}

func (t *HTTPClientBuilderTest) Test_FromEngine() {
	engine := &Engine{
		Config: Config{
			HTTPClientConfig: &HTTPClientConfig{
				SSLVerification: boolPtr(true),
				MockAddress:     "anothermock.local:3004",
				MockedDomains:   []string{"example.gov"},
			},
			Debug: false,
		},
	}

	t.builder.FromEngine(engine)
	assert.Equal(t.T(), engine.Config.GetHTTPClientConfig().MockAddress, t.builder.mockAddress)
}

func (t *HTTPClientBuilderTest) Test_buildDialer() {
	t.builder.buildDialer()

	assert.NotNil(t.T(), t.builder.dialer)
}

func (t *HTTPClientBuilderTest) Test_parseAddress() {
	assert.NoError(t.T(), t.builder.parseAddress())
}

func (t *HTTPClientBuilderTest) Test_buildMocks() {
	assert.NoError(t.T(), t.builder.buildMocks())
}

func (t *HTTPClientBuilderTest) Test_WithLogger() {
	logger := logger.NewStandard("telegram", logging.ERROR, logger.DefaultLogFormatter())
	builder := NewHTTPClientBuilder()
	require.Nil(t.T(), builder.logger)

	builder.WithLogger(nil)
	assert.Nil(t.T(), builder.logger)

	builder.WithLogger(logger)
	assert.NotNil(t.T(), builder.logger)
}

func (t *HTTPClientBuilderTest) Test_logf() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	t.builder.logf("test %s", "string")
}

func (t *HTTPClientBuilderTest) Test_Build() {
	timeout := time.Duration(10)
	pool := x509.NewCertPool()
	client, err := t.builder.
		SetTimeout(timeout).
		SetMockAddress("api_mock:3004").
		AddMockedDomain("google.com").
		SetCertPool(pool).
		Build(true)

	assert.NoError(t.T(), err)
	assert.NotNil(t.T(), client)
	assert.Equal(t.T(), client, http.DefaultClient)
	assert.Equal(t.T(), timeout*time.Second, client.Timeout)
	assert.Equal(t.T(), pool, client.Transport.(*http.Transport).TLSClientConfig.RootCAs)
}

func (t *HTTPClientBuilderTest) Test_RestoreDefault() {
	t.builder.ReplaceDefault()
	t.builder.RestoreDefault()

	assert.Equal(t.T(), http.DefaultClient, DefaultClient)
	assert.Equal(t.T(), http.DefaultTransport, DefaultTransport)
}

// Test_ClientMocksWorking is supposed to test mocking functionality of generated client.
// Using real HTTP requests and server doesn't look good obviously, but mocking something to check how
// mocks are working doesn't look that good too.
// In this case we are trying to test this in just-like-real environment, without mocks or etc.
// Fake "real" environment is easiest way to do this.
// You know how to make this test better? Let us know.
func (t *HTTPClientBuilderTest) Test_ClientMocksWorking() {
	mockProto := "https://"
	mockServerAddr := getOutboundIP().String() + ":27717"
	mockDomainAddr := "example.com"
	certFileData := `-----BEGIN CERTIFICATE-----
MIIC+TCCAeGgAwIBAgIJAPb0Qm9aV+93MA0GCSqGSIb3DQEBBQUAMBMxETAPBgNV
BAMMCGFwaV9tb2NrMB4XDTE5MTAxNzE0NDMzMloXDTI5MTAxNDE0NDMzMlowEzER
MA8GA1UEAwwIYXBpX21vY2swggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB
AQCtxUg6aUBcZLjEYEgzzE+B8wAsyK6dT9fbbDfO/5n9PhHjxQLnjTYrs9xAAl7R
Caj8nUg0RzfX38jD1TGwkFrC1u1pjOf74OVMXsw2xa7gmZlnJeeL+QozXQX1rDPk
wO5QqomKAIwM3ab+i6k1tLBfDIOHGLhTEFQZ9cmKVuNdTlkeqBh+bKduRIr7DYhQ
Dsci/PacGJlt0W+r2YuRmm1KGearbS4HabPOkC0c6KbVD+bUyF+F7DxtJ8vg7O4W
SWAXlwoEHonIyJG8H7TQxL2g5w/4UQhp6awAzFNLhqtf/6pw6gPfI9joS+Z/Pxvz
Bry41s4LV7KP8v0GqRK3KH2rAgMBAAGjUDBOMB0GA1UdDgQWBBTEiHl+R8N0kLwH
1RTsKYe8joAwxzAfBgNVHSMEGDAWgBTEiHl+R8N0kLwH1RTsKYe8joAwxzAMBgNV
HRMEBTADAQH/MA0GCSqGSIb3DQEBBQUAA4IBAQAo/XBUFlrl5yuQ+5eFQjMmbn4T
9qVg4NVRy5qnTKzcOR21X3fB394TvfBiGstW0YQCOCtNakv94UAbZozYktQaYOtP
x5porosgI2RgOTTwmiYOcYQTS2650jYydHhK16Gu2b3UKernO16mAWXNDWfvS2bk
1ufbRWpuUXs0SIR6e/mgSwrBMBvq6fan4EVdEkx4Catjna15DgmBGRL215t5K4aq
nAI2GL2ACEdOCyRvgq16AycJJYU7nYQ+t9aveefx0uhbYYIVeYub9NxmCfD3MojI
saG/63vo0ng851n90DVoMRWx9n1CjEvss/vvz+jXIl9njaCtizN3WUf1NwUB
-----END CERTIFICATE-----`
	keyFileData := `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEArcVIOmlAXGS4xGBIM8xPgfMALMiunU/X22w3zv+Z/T4R48UC
5402K7PcQAJe0Qmo/J1INEc319/Iw9UxsJBawtbtaYzn++DlTF7MNsWu4JmZZyXn
i/kKM10F9awz5MDuUKqJigCMDN2m/oupNbSwXwyDhxi4UxBUGfXJilbjXU5ZHqgY
fmynbkSK+w2IUA7HIvz2nBiZbdFvq9mLkZptShnmq20uB2mzzpAtHOim1Q/m1Mhf
hew8bSfL4OzuFklgF5cKBB6JyMiRvB+00MS9oOcP+FEIaemsAMxTS4arX/+qcOoD
3yPY6Evmfz8b8wa8uNbOC1eyj/L9BqkStyh9qwIDAQABAoIBAG/jNUyW9KAJIocf
T546kX8kzhoH5ZbZEC3ykkSwg6Bx1OcJtYMOg7DEEK8OV6rVQ3/Ubedra+ON7iFa
JrJ/YFFQPnHWDpE6D6qK54bk5mMrw4CNAXg5FH5aCTpUdN139HrwM7st+v9VwO7p
UjyIaX/p+M6F9jlVrDFC91Ah/ifW+DzWS27IsQ/396HTvTUNgD/Lj9pKtC6eW3AQ
aTpLk/cVrkxxT1qc4rx8NRPe9/670CQDQYMvaNJtC54kRUzH87mn06IEy5fuLeI4
WwYgEIkh1YjQsJhg3Z788JGKR5iKuzjXpw7aD6QESISoWPW1Dh/5PJO1zkKxZril
xUepcgECgYEA5MLP4oCiuloShhaMc+UMYgH8YuglXegGbF5/iG9d7kgaiz4KiXuD
ObwaHegLDFNSxtVSiH6AsXErzd7HHojzTm6B3O5qtBl+QgpW9hiTyJnCxkdzxK7y
cVan8Jp3g/ojcYBY+QgZlR81QHQIwSUrAbMC0fjYlrKb5zJfExgFv4ECgYEAwnY7
MUoLsHs2eXVOekvb42ReCq7HKVz8TETlSEPN4B5XzOKCnXxCUUvF5iWjWj0XAMSU
yF6LJmmfFmOaVHpXDHhF9MmNKxYiIISS+ZM6B2DXP+hS/DXXuy5hGmYO6p4JC08d
qulIIR7JSJmHyI+5Ref40WTObJvJQDXi2p5a0ysCgYBjdONG2aBmHrUBARqtZH7e
uXhOVBmy2ya3xNnzql+PMl//+8g+/6kM1+AO8oyjHjLV6XcJit5OxyJBTkMJ3obR
qa/iKvHPPWosMiyesA7IXzlUVUpaz6juZ7t6Gt4tTfpM5X1JQCFHORtA23HW717k
TTzDp0obMqofeUHmnkIZgQKBgFBaGktblUjvIKs/VZYjElD7gABaB+GHkpjRPwyF
N+SLpSv7zIzWc3C0Jqnak40OARtIH1JL/qN4sUvHDFYr1xxH9mAXiEVtd9yH61NF
Co1R7p9xmBivBt1JZMZLtY4sjwAlSNT+X9ePqQxepESzXpMMLzwWs1UdaiMmIP7E
wDLRAoGAM/Cz7B+J7KcKI8VqXAsX5nMklIKvACScKU6oIPwNYXyxUfm7jsNaBqXG
weywTxDl/OD5ybNkZIRKsIXciFYG1VCGO2HNGN9qJcV+nJ63kyrIBauwUkuEhiN5
uf/TQPpjrGW5nxOf94qn6FzV2WSype9BcM5MD7z7rk202Fs7Zqc=
-----END RSA PRIVATE KEY-----`

	certFile, err := ioutil.TempFile("/tmp", "cert_")
	require.NoError(t.T(), err, "cannot create temp cert file")
	keyFile, err := ioutil.TempFile("/tmp", "key_")
	require.NoError(t.T(), err, "cannot create temp key file")

	_, err = certFile.WriteString(certFileData)
	require.NoError(t.T(), err, "cannot write temp cert file")
	_, err = keyFile.WriteString(keyFileData)
	require.NoError(t.T(), err, "cannot write temp key file")
	require.NoError(t.T(),
		errorutil.Collect(certFile.Sync(), certFile.Close()), "cannot sync and close temp cert file")
	require.NoError(t.T(),
		errorutil.Collect(keyFile.Sync(), keyFile.Close()), "cannot sync and close temp key file")

	mux := &http.ServeMux{}
	srv := &http.Server{Addr: mockServerAddr, Handler: mux}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "ok")
	})

	testSkipChan := make(chan error, 1)
	go func(skip chan error) {
		if err := srv.ListenAndServeTLS(certFile.Name(), keyFile.Name()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			skip <- fmt.Errorf("skipping test because server won't start: %w", err)
		}
	}(testSkipChan)

	select {
	case errStartup := <-testSkipChan:
		t.T().Skip(errStartup)
		return
	case <-time.After(time.Second):
		t.T().Log("test server started")
	}

	defer func() {
		if err := srv.Shutdown(context.TODO()); err != nil {
			t.T().Log("warning > cannot shutdown server gracefully: ", err)
		} else {
			t.T().Log("test server stopped")
		}

		if err := os.Remove(certFile.Name()); err != nil {
			t.T().Log("warning > cannot remove temp cert file properly: ", err)
		}

		if err := os.Remove(keyFile.Name()); err != nil {
			t.T().Log("warning > cannot remove temp key file properly: ", err)
		}
	}()

	client, err := NewHTTPClientBuilder().
		SetLogging(true).
		SetMockAddress(mockServerAddr).
		SetMockedDomains([]string{mockDomainAddr}).
		SetTimeout(time.Second).
		SetSSLVerification(false).
		Build()
	require.NoError(t.T(), err, "cannot build client")

	resp, err := client.Get(mockProto + mockDomainAddr)
	if err != nil && strings.Contains(err.Error(), "connection refused") {
		t.T().Skip("connection refused - skipping test: ", err)
	}
	require.NoError(t.T(), err, "error while making request")

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	require.NoError(t.T(), err, "error while reading body")

	assert.Equal(t.T(), http.StatusCreated, resp.StatusCode, "invalid status code")
	assert.Equal(t.T(), "ok", string(data), "invalid body contents")
}

func (t *HTTPClientBuilderTest) Test_UseTLS10() {
	client, err := NewHTTPClientBuilder().SetSSLVerification(true).UseTLS10().Build()

	t.Require().NoError(err)
	t.Require().NotNil(client)
	t.Require().NotNil(client.Transport)
	t.Require().NotNil(client.Transport.(*http.Transport).TLSClientConfig)
	t.Assert().Equal(uint16(tls.VersionTLS10), client.Transport.(*http.Transport).TLSClientConfig.MinVersion)
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
