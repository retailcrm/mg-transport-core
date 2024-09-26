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

func (s *TestDefaultSuite) TestForHandlerNoDuplicate() {
	log := newBufferLogger()
	log.ForHandler("handler1").ForHandler("handler2").Info("test")

	s.Assert().Contains(log.String(), "handler2")
	s.Assert().NotContains(log.String(), "handler1")
}

func (s *TestDefaultSuite) TestForConnection() {
	log := newBufferLogger()
	log.ForConnection("connection").Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("connection", items[0].Connection)
}

func (s *TestDefaultSuite) TestForConnectionNoDuplicate() {
	log := newBufferLogger()
	log.ForConnection("conn1").ForConnection("conn2").Info("test")

	s.Assert().Contains(log.String(), "conn2")
	s.Assert().NotContains(log.String(), "conn1")
}

func (s *TestDefaultSuite) TestForAccount() {
	log := newBufferLogger()
	log.ForAccount("account").Info("test")
	items, err := newJSONBufferedLogger(log).ScanAll()

	s.Require().NoError(err)
	s.Require().Len(items, 1)
	s.Assert().Equal("account", items[0].Account)
}

func (s *TestDefaultSuite) TestForAccountNoDuplicate() {
	log := newBufferLogger()
	log.ForAccount("acc1").ForAccount("acc2").Info("test")

	s.Assert().Contains(log.String(), "acc2")
	s.Assert().NotContains(log.String(), "acc1")
}

func (s *TestDefaultSuite) TestNoDuplicatesPersistRecords() {
	log := newBufferLogger()
	log.
		ForHandler("handler1").
		ForHandler("handler2").
		ForConnection("conn1").
		ForConnection("conn2").
		ForAccount("acc1").
		ForAccount("acc2").
		Info("test")

	s.Assert().Contains(log.String(), "handler2")
	s.Assert().NotContains(log.String(), "handler1")
	s.Assert().Contains(log.String(), "conn2")
	s.Assert().NotContains(log.String(), "conn1")
	s.Assert().Contains(log.String(), "acc2")
	s.Assert().NotContains(log.String(), "acc1")
}

// TestPersistRecordsIncompatibleWith is not a unit test, but rather a demonstration how you shouldn't use For* methods.
func (s *TestDefaultSuite) TestPersistRecordsIncompatibleWith() {
	log := newBufferLogger()
	log.
		ForHandler("handler1").
		With(zap.Int("f1", 1)).
		ForHandler("handler2").
		ForConnection("conn1").
		With(zap.Int("f2", 2)).
		ForConnection("conn2").
		ForAccount("acc1").
		With(zap.Int("f3", 3)).
		ForAccount("acc2").
		Info("test")

	s.Assert().Contains(log.String(), "handler2")
	s.Assert().Contains(log.String(), "handler1")
	s.Assert().Contains(log.String(), "conn2")
	s.Assert().Contains(log.String(), "conn1")
	s.Assert().Contains(log.String(), "acc2")
	s.Assert().Contains(log.String(), "acc1")
}

func TestAnyZapFields(t *testing.T) {
	fields := AnyZapFields([]interface{}{zap.String("k0", "v0"), "ooga", "booga"})
	require.Len(t, fields, 3)
	assert.Equal(t, zap.String("k0", "v0"), fields[0])
	assert.Equal(t, zap.String("arg1", "ooga"), fields[1])
	assert.Equal(t, zap.String("arg2", "booga"), fields[2])
}
