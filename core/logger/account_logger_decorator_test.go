package logger

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	testComponent    = "ComponentName"
	testConnectionID = "https://test.retailcrm.pro"
	testAccountID    = "@account_name"
)

type AccountLoggerDecoratorTest struct {
	suite.Suite
	buf    *bytes.Buffer
	logger AccountLogger
}

func TestAccountLoggerDecorator(t *testing.T) {
	suite.Run(t, new(AccountLoggerDecoratorTest))
}

func TestNewForAccount(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewForAccount("code", "component", "conn", "acc", logging.DEBUG, DefaultLogFormatter())
	logger.(*AccountLoggerDecorator).backend.(*StandardLogger).
		SetBaseLogger(NewBase(buf, "code", logging.DEBUG, DefaultLogFormatter()))
	logger.Debugf("message %s", "text")

	assert.Contains(t, buf.String(), fmt.Sprintf(DefaultAccountLoggerFormat+" message", "component", "conn", "acc"))
}

func (t *AccountLoggerDecoratorTest) SetupSuite() {
	t.buf = &bytes.Buffer{}
	t.logger = DecorateForAccount((&StandardLogger{}).
		SetBaseLogger(NewBase(t.buf, "code", logging.DEBUG, DefaultLogFormatter())),
		testComponent, testConnectionID, testAccountID)
}

func (t *AccountLoggerDecoratorTest) SetupTest() {
	t.buf.Reset()
	t.logger.SetComponent(testComponent)
	t.logger.SetConnectionIdentifier(testConnectionID)
	t.logger.SetAccountIdentifier(testAccountID)
	t.logger.SetPrefixFormat(DefaultAccountLoggerFormat)
}

func (t *AccountLoggerDecoratorTest) Test_LogWithNewFormat() {
	t.logger.SetPrefixFormat("[%s (%s: %s)] =>")
	t.logger.Infof("test message")

	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(),
		fmt.Sprintf("[%s (%s: %s)] =>", testComponent, testConnectionID, testAccountID))
}

func (t *AccountLoggerDecoratorTest) Test_Log() {
	t.logger.Infof("test message")
	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(),
		fmt.Sprintf(DefaultAccountLoggerFormat, testComponent, testConnectionID, testAccountID))
}

func (t *AccountLoggerDecoratorTest) Test_SetComponent() {
	t.logger.SetComponent("NewComponent")
	t.logger.Infof("test message")

	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(),
		fmt.Sprintf(DefaultAccountLoggerFormat, "NewComponent", testConnectionID, testAccountID))
}

func (t *AccountLoggerDecoratorTest) Test_SetConnectionIdentifier() {
	t.logger.SetComponent("NewComponent")
	t.logger.SetConnectionIdentifier("https://test.simla.com")
	t.logger.Infof("test message")

	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(),
		fmt.Sprintf(DefaultAccountLoggerFormat, "NewComponent", "https://test.simla.com", testAccountID))
}

func (t *AccountLoggerDecoratorTest) Test_SetAccountIdentifier() {
	t.logger.SetComponent("NewComponent")
	t.logger.SetConnectionIdentifier("https://test.simla.com")
	t.logger.SetAccountIdentifier("@new_account_name")
	t.logger.Infof("test message")

	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(),
		fmt.Sprintf(DefaultAccountLoggerFormat, "NewComponent", "https://test.simla.com", "@new_account_name"))
}
