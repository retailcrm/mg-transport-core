package logger

import (
	"bytes"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type StandardLoggerTest struct {
	suite.Suite
	logger *StandardLogger
	buf    *bytes.Buffer
}

func TestLogger_NewLogger(t *testing.T) {
	logger := NewStandard("code", logging.DEBUG, DefaultLogFormatter())
	assert.NotNil(t, logger)
}

func TestLogger_DefaultLogFormatter(t *testing.T) {
	formatter := DefaultLogFormatter()

	assert.NotNil(t, formatter)
	assert.IsType(t, logging.MustStringFormatter(`%{message}`), formatter)
}

func Test_Logger(t *testing.T) {
	suite.Run(t, new(StandardLoggerTest))
}

func (t *StandardLoggerTest) SetupSuite() {
	t.buf = &bytes.Buffer{}
	t.logger = (&StandardLogger{}).
		Exclusive().
		SetBaseLogger(NewBase(t.buf, "code", logging.DEBUG, DefaultLogFormatter()))
}

func (t *StandardLoggerTest) SetupTest() {
	t.buf.Reset()
}

// TODO Cover Fatal and Fatalf (implementation below is no-op)
// func (t *StandardLoggerTest) Test_Fatal() {
// 	if os.Getenv("FLAG") == "1" {
// 		t.logger.Fatal("test", "fatal")
// 		return
// 	}

// 	cmd := exec.Command(os.Args[0], "-test.run=TestGetConfig")
// 	cmd.Env = append(os.Environ(), "FLAG=1")
// 	err := cmd.Run()

// 	e, ok := err.(*exec.ExitError)
// 	expectedErrorString := "test fatal"
// 	t.Assert().Equal(true, ok)
// 	t.Assert().Equal(expectedErrorString, e.Error())
// }

func (t *StandardLoggerTest) Test_Panic() {
	defer func() {
		t.Assert().NotNil(recover())
		t.Assert().Contains(t.buf.String(), "CRIT")
		t.Assert().Contains(t.buf.String(), "panic")
	}()
	t.logger.Panic("panic")
}

func (t *StandardLoggerTest) Test_Panicf() {
	defer func() {
		t.Assert().NotNil(recover())
		t.Assert().Contains(t.buf.String(), "CRIT")
		t.Assert().Contains(t.buf.String(), "panicf")
	}()
	t.logger.Panicf("panicf")
}

func (t *StandardLoggerTest) Test_Critical() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "CRIT")
		t.Assert().Contains(t.buf.String(), "critical")
	}()
	t.logger.Critical("critical")
}

func (t *StandardLoggerTest) Test_Criticalf() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "CRIT")
		t.Assert().Contains(t.buf.String(), "critical")
	}()
	t.logger.Criticalf("critical")
}

func (t *StandardLoggerTest) Test_Warning() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "WARN")
		t.Assert().Contains(t.buf.String(), "warning")
	}()
	t.logger.Warning("warning")
}

func (t *StandardLoggerTest) Test_Notice() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "NOTI")
		t.Assert().Contains(t.buf.String(), "notice")
	}()
	t.logger.Notice("notice")
}

func (t *StandardLoggerTest) Test_Info() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "INFO")
		t.Assert().Contains(t.buf.String(), "info")
	}()
	t.logger.Info("info")
}

func (t *StandardLoggerTest) Test_Debug() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "DEBU")
		t.Assert().Contains(t.buf.String(), "debug")
	}()
	t.logger.Debug("debug")
}

func (t *StandardLoggerTest) Test_Warningf() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "WARN")
		t.Assert().Contains(t.buf.String(), "warning")
	}()
	t.logger.Warningf("%s", "warning")
}

func (t *StandardLoggerTest) Test_Noticef() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "NOTI")
		t.Assert().Contains(t.buf.String(), "notice")
	}()
	t.logger.Noticef("%s", "notice")
}

func (t *StandardLoggerTest) Test_Infof() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "INFO")
		t.Assert().Contains(t.buf.String(), "info")
	}()
	t.logger.Infof("%s", "info")
}

func (t *StandardLoggerTest) Test_Debugf() {
	defer func() {
		t.Require().Nil(recover())
		t.Assert().Contains(t.buf.String(), "DEBU")
		t.Assert().Contains(t.buf.String(), "debug")
	}()
	t.logger.Debugf("%s", "debug")
}
