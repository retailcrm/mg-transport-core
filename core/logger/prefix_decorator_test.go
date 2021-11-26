package logger

import (
	"bytes"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const testPrefix = "TestPrefix:"

type PrefixDecoratorTest struct {
	suite.Suite
	buf    *bytes.Buffer
	logger PrefixedLogger
}

func TestPrefixDecorator(t *testing.T) {
	suite.Run(t, new(PrefixDecoratorTest))
}

func TestNewWithPrefix(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewWithPrefix("code", "Prefix:", logging.DEBUG, DefaultLogFormatter())
	logger.(*PrefixDecorator).backend.(*StandardLogger).
		SetBaseLogger(NewBase(buf, "code", logging.DEBUG, DefaultLogFormatter()))
	logger.Debugf("message %s", "text")

	assert.Contains(t, buf.String(), "Prefix: message text")
}

func (t *PrefixDecoratorTest) SetupSuite() {
	t.buf = &bytes.Buffer{}
	t.logger = DecorateWithPrefix((&StandardLogger{}).
		SetBaseLogger(NewBase(t.buf, "code", logging.DEBUG, DefaultLogFormatter())), testPrefix)
}

func (t *PrefixDecoratorTest) SetupTest() {
	t.buf.Reset()
	t.logger.SetPrefix(testPrefix)
}

func (t *PrefixDecoratorTest) Test_SetPrefix() {
	t.logger.Info("message")
	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")

	t.logger.SetPrefix(testPrefix + testPrefix)
	t.logger.Info("message")
	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(), testPrefix+testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Panic() {
	t.Require().Panics(func() {
		t.logger.Panic("message")
	})
	t.Assert().Contains(t.buf.String(), "CRIT")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Panicf() {
	t.Require().Panics(func() {
		t.logger.Panicf("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "CRIT")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Critical() {
	t.Require().NotPanics(func() {
		t.logger.Critical("message")
	})
	t.Assert().Contains(t.buf.String(), "CRIT")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Criticalf() {
	t.Require().NotPanics(func() {
		t.logger.Criticalf("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "CRIT")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Error() {
	t.Require().NotPanics(func() {
		t.logger.Error("message")
	})
	t.Assert().Contains(t.buf.String(), "ERRO")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Errorf() {
	t.Require().NotPanics(func() {
		t.logger.Errorf("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "ERRO")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Warning() {
	t.Require().NotPanics(func() {
		t.logger.Warning("message")
	})
	t.Assert().Contains(t.buf.String(), "WARN")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Warningf() {
	t.Require().NotPanics(func() {
		t.logger.Warningf("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "WARN")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Notice() {
	t.Require().NotPanics(func() {
		t.logger.Notice("message")
	})
	t.Assert().Contains(t.buf.String(), "NOTI")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Noticef() {
	t.Require().NotPanics(func() {
		t.logger.Noticef("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "NOTI")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Info() {
	t.Require().NotPanics(func() {
		t.logger.Info("message")
	})
	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Infof() {
	t.Require().NotPanics(func() {
		t.logger.Infof("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "INFO")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Debug() {
	t.Require().NotPanics(func() {
		t.logger.Debug("message")
	})
	t.Assert().Contains(t.buf.String(), "DEBU")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}

func (t *PrefixDecoratorTest) Test_Debugf() {
	t.Require().NotPanics(func() {
		t.logger.Debugf("%s", "message")
	})
	t.Assert().Contains(t.buf.String(), "DEBU")
	t.Assert().Contains(t.buf.String(), testPrefix+" message")
}
