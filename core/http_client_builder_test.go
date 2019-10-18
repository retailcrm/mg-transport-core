package core

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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

func (t *HTTPClientBuilderTest) Test_FromConfig() {
	config := &HTTPClientConfig{
		SSLVerification: true,
		MockAddress:     "anothermock.local:3004",
		MockedDomains:   []string{"example.gov"},
		Timeout:         60,
	}

	t.builder.FromConfig(config)
	assert.Equal(t.T(), !config.SSLVerification, t.builder.httpTransport.TLSClientConfig.InsecureSkipVerify)
	assert.Equal(t.T(), config.MockAddress, t.builder.mockAddress)
	assert.Equal(t.T(), config.MockedDomains[0], t.builder.mockedDomains[0])
	assert.Equal(t.T(), config.Timeout*time.Second, t.builder.timeout)
	assert.Equal(t.T(), config.Timeout*time.Second, t.builder.httpClient.Timeout)
}

func (t *HTTPClientBuilderTest) Test_FromEngine() {
	engine := &Engine{
		Config: Config{
			HTTPClientConfig: &HTTPClientConfig{
				SSLVerification: true,
				MockAddress:     "anothermock.local:3004",
				MockedDomains:   []string{"example.gov"},
			},
			Debug: false,
		},
	}

	t.builder.FromEngine(engine)
	assert.NotNil(t.T(), engine, t.builder.engine)
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

func (t *HTTPClientBuilderTest) Test_logf() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	t.builder.logf("test %s", "string")
}

func (t *HTTPClientBuilderTest) Test_Build() {
	client, err := t.builder.
		SetTimeout(10).
		SetMockAddress("api_mock:3004").
		AddMockedDomain("google.com").
		Build(true)

	assert.NoError(t.T(), err)
	assert.NotNil(t.T(), client)
	assert.Equal(t.T(), client, http.DefaultClient)
}

func (t *HTTPClientBuilderTest) Test_RestoreDefault() {
	t.builder.ReplaceDefault()
	t.builder.RestoreDefault()

	assert.Equal(t.T(), http.DefaultClient, DefaultClient)
	assert.Equal(t.T(), http.DefaultTransport, DefaultTransport)
}

func Test_HTTPClientBuilder(t *testing.T) {
	suite.Run(t, new(HTTPClientBuilderTest))
}
