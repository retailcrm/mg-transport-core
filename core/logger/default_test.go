package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type TestDefaultSuite struct {
	suite.Suite
}

func TestDefault(t *testing.T) {
	suite.Run(t, new(TestDefaultSuite))
}

func (s *TestDefaultSuite) TestNewDefault_OK() {
	jsonLog := NewDefault("json", false)
	consoleLog := NewDefault("console", true)

	s.Assert().NotNil(jsonLog)
	s.Assert().NotNil(consoleLog)
}

func (s *TestDefaultSuite) TestNewDefault_Panic() {
	s.Assert().PanicsWithValue("unknown logger format: rar", func() {
		NewDefault("rar", false)
	})
}

func (s *TestDefaultSuite) TestWith() {
	log := newBufferLogger()
	log.With(zap.String(HandlerAttr, "Handler")).Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("Handler", items[0].Handler)
}

func (s *TestDefaultSuite) TestWithLazy() {
	log := newBufferLogger()
	log.WithLazy(zap.String(HandlerAttr, "Handler")).Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("Handler", items[0].Handler)
}

func (s *TestDefaultSuite) TestForHandler() {
	log := newBufferLogger()
	log.ForHandler("Handler").Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("Handler", items[0].Handler)
}

func (s *TestDefaultSuite) TestForConnection() {
	log := newBufferLogger()
	log.ForConnection("connection").Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("connection", items[0].Connection)
}

func (s *TestDefaultSuite) TestForAccount() {
	log := newBufferLogger()
	log.ForAccount("account").Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("account", items[0].Account)
}

func TestAnyZapFields(t *testing.T) {
	fields := AnyZapFields([]interface{}{zap.String("k0", "v0"), "ooga", "booga"})
	require.Len(t, fields, 3)
	assert.Equal(t, zap.String("k0", "v0"), fields[0])
	assert.Equal(t, zap.String("arg1", "ooga"), fields[1])
	assert.Equal(t, zap.String("arg2", "booga"), fields[2])
}
