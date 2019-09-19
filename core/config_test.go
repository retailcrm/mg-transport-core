package core

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var testConfigFile = path.Join(os.TempDir(), "config_test.yml")

type ConfigTest struct {
	suite.Suite
	config *Config
	data   []byte
}

func (c *ConfigTest) SetupTest() {
	c.data = []byte(`
version: 3.2.1

database:
    connection: postgres://user:password@host:5432/dbname?sslmode=disable

http_server:
    host: example.com
    listen: :3001

transport_info:
    name: Transport
    code: mg-transport
    logo_path: /static/logo.svg

sentry_dsn: dsn string
log_level: 5
debug: true
update_interval: 24

config_aws:
    access_key_id: key
    secret_access_key: secret
    region: region
    bucket: bucket
    folder_name: folder
    content_type: image/jpeg`)
	err := ioutil.WriteFile(testConfigFile, c.data, os.ModePerm)
	require.Nil(c.T(), err)

	c.config = NewConfig(testConfigFile)
}

func (c *ConfigTest) Test_GetConfigData() {
	assert.Equal(c.T(), c.data, c.config.GetConfigData(testConfigFile))
}

func (c *ConfigTest) Test_GetVersion() {
	assert.Equal(c.T(), "3.2.1", c.config.GetVersion())
}

func (c *ConfigTest) Test_GetDBConfig() {
	assert.Equal(c.T(), "postgres://user:password@host:5432/dbname?sslmode=disable", c.config.GetDBConfig().Connection)
}

func (c *ConfigTest) Test_GetHttpServer() {
	assert.Equal(c.T(), "example.com", c.config.GetHTTPConfig().Host)
	assert.Equal(c.T(), ":3001", c.config.GetHTTPConfig().Listen)
}

func (c *ConfigTest) Test_GetTransportInfo() {
	assert.Equal(c.T(), "Transport", c.config.GetTransportInfo().GetName())
	assert.Equal(c.T(), "mg-transport", c.config.GetTransportInfo().GetCode())
	assert.Equal(c.T(), "/static/logo.svg", c.config.GetTransportInfo().GetLogoPath())
}

func (c *ConfigTest) Test_GetSentryDSN() {
	assert.Equal(c.T(), "dsn string", c.config.GetSentryDSN())
}

func (c *ConfigTest) Test_GetLogLevel() {
	assert.Equal(c.T(), logging.Level(5), c.config.GetLogLevel())
}

func (c *ConfigTest) Test_IsDebug() {
	assert.Equal(c.T(), true, c.config.IsDebug())
}

func (c *ConfigTest) Test_GetUpdateInterval() {
	assert.Equal(c.T(), 24, c.config.GetUpdateInterval())
}

func (c *ConfigTest) Test_GetConfigAWS() {
	assert.Equal(c.T(), "key", c.config.GetAWSConfig().AccessKeyID)
	assert.Equal(c.T(), "secret", c.config.GetAWSConfig().SecretAccessKey)
	assert.Equal(c.T(), "region", c.config.GetAWSConfig().Region)
	assert.Equal(c.T(), "bucket", c.config.GetAWSConfig().Bucket)
	assert.Equal(c.T(), "folder", c.config.GetAWSConfig().FolderName)
	assert.Equal(c.T(), "image/jpeg", c.config.GetAWSConfig().ContentType)
}

func (c *ConfigTest) TearDownTest() {
	_ = os.Remove(testConfigFile)
}

func TestConfig_Suite(t *testing.T) {
	suite.Run(t, new(ConfigTest))
}

func TestConfig_NoFile(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	_ = NewConfig(path.Join(os.TempDir(), "file_which_should_not_exist_anyway"))
}
