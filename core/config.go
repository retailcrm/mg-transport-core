package core

import (
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

var (
	credentialsTransport = []string{
		"/api/integration-modules/{code}",
		"/api/integration-modules/{code}/edit",
	}
	markdownSymbols = []string{"*", "_", "`", "["}
	regCommandName  = regexp.MustCompile(`^https://?[\da-z.-]+\.(retailcrm\.(ru|pro|es)|ecomlogic\.com|simlachat\.(com|ru))/?$`)
	slashRegex      = regexp.MustCompile(`/+$`)
)


// ConfigInterface settings data structure
type ConfigInterface interface {
	GetVersion() string
	GetSentryDSN() string
	GetLogLevel() logging.Level
	GetDebug() bool
	GetHTTPConfig() HTTPServerConfig
	GetDBConfig() DatabaseConfig
	GetAWSConfig() ConfigAWS
	GetTransportInfo() InfoInterface
	GetUpdateInterval() int
	IsDebug() bool
}

// InfoInterface transport settings data structure
type InfoInterface interface {
	GetName() string
	GetCode() string
	GetLogoPath() string
}

// Config struct
type Config struct {
	Version        string           `yaml:"version"`
	LogLevel       logging.Level    `yaml:"log_level"`
	Database       DatabaseConfig   `yaml:"database"`
	SentryDSN      string           `yaml:"sentry_dsn"`
	HTTPServer     HTTPServerConfig `yaml:"http_server"`
	Debug          bool             `yaml:"debug"`
	UpdateInterval int              `yaml:"update_interval"`
	ConfigAWS      ConfigAWS        `yaml:"config_aws"`
	TransportInfo  Info             `yaml:"transport_info"`
}

// Info struct
type Info struct {
	Name     string `yaml:"name"`
	Code     string `yaml:"code"`
	LogoPath string `yaml:"logo_path"`
}

// ConfigAWS struct
type ConfigAWS struct {
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Region          string `yaml:"region"`
	Bucket          string `yaml:"bucket"`
	FolderName      string `yaml:"folder_name"`
	ContentType     string `yaml:"content_type"`
}

// DatabaseConfig struct
type DatabaseConfig struct {
	Connection         string `yaml:"connection"`
	Logging            bool   `yaml:"logging"`
	TablePrefix        string `yaml:"table_prefix"`
	MaxOpenConnections int    `yaml:"max_open_connections"`
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	ConnectionLifetime int    `yaml:"connection_lifetime"`
}

// HTTPServerConfig struct
type HTTPServerConfig struct {
	Host   string `yaml:"host"`
	Listen string `yaml:"listen"`
}

// NewConfig reads configuration file and returns config instance
// Usage:
//      NewConfig("config.yml")
func NewConfig(path string) *Config {
	return (&Config{}).LoadConfig(path)
}

// LoadConfig read & load configuration file
func (c *Config) LoadConfig(path string) *Config {
	var err error

	path, err = filepath.Abs(path)
	if err != nil {
		panic(err)
	}

	source, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	if err = yaml.Unmarshal(source, c); err != nil {
		panic(err)
	}

	return c
}

// GetSentryDSN sentry connection dsn
func (c Config) GetSentryDSN() string {
	return c.SentryDSN
}

// GetVersion transport version
func (c Config) GetVersion() string {
	return c.Version
}

// GetLogLevel log level
func (c Config) GetLogLevel() logging.Level {
	return c.LogLevel
}

// GetTransportInfo transport basic data
func (c Config) GetTransportInfo() InfoInterface {
	return c.TransportInfo
}

// GetDebug debug flag
func (c Config) GetDebug() bool {
	return c.Debug
}

// GetAWSConfig AWS configuration
func (c Config) GetAWSConfig() ConfigAWS {
	return c.ConfigAWS
}

// GetDBConfig database configuration
func (c Config) GetDBConfig() DatabaseConfig {
	return c.Database
}

// GetHTTPConfig server configuration
func (c Config) GetHTTPConfig() HTTPServerConfig {
	return c.HTTPServer
}

// GetUpdateInterval user data update interval
func (c Config) GetUpdateInterval() int {
	return c.UpdateInterval
}

// IsDebug debug state
func (c Config) IsDebug() bool {
	return c.Debug
}

// GetName transport name
func (t Info) GetName() string {
	return t.Name
}

// GetCode transport code
func (t Info) GetCode() string {
	return t.Code
}

// GetLogoPath transport logo
func (t Info) GetLogoPath() string {
	return t.LogoPath
}
