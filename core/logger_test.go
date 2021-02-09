package core

import (
	// "os"
	// "os/exec".
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LoggerTest struct {
	suite.Suite
	logger *Logger
}

func TestLogger_NewLogger(t *testing.T) {
	logger := NewLogger("code", logging.DEBUG, DefaultLogFormatter())
	assert.NotNil(t, logger)
}

func TestLogger_DefaultLogFormatter(t *testing.T) {
	formatter := DefaultLogFormatter()

	assert.NotNil(t, formatter)
	assert.IsType(t, logging.MustStringFormatter(`%{message}`), formatter)
}

func Test_Logger(t *testing.T) {
	suite.Run(t, new(LoggerTest))
}

func (t *LoggerTest) SetupSuite() {
	t.logger = NewLogger("code", logging.DEBUG, DefaultLogFormatter()).Exclusive()
}

// TODO Cover Fatal and Fatalf (implementation below is no-op)
// func (t *LoggerTest) Test_Fatal() {
// 	if os.Getenv("FLAG") == "1" {
// 		t.logger.Fatal("test", "fatal")
// 		return
// 	}

// 	cmd := exec.Command(os.Args[0], "-test.run=TestGetConfig")
// 	cmd.Env = append(os.Environ(), "FLAG=1")
// 	err := cmd.Run()

// 	e, ok := err.(*exec.ExitError)
// 	expectedErrorString := "test fatal"
// 	assert.Equal(t.T(), true, ok)
// 	assert.Equal(t.T(), expectedErrorString, e.Error())
// }

func (t *LoggerTest) Test_Panic() {
	defer func() {
		assert.NotNil(t.T(), recover())
	}()
	t.logger.Panic("panic")
}

func (t *LoggerTest) Test_Panicf() {
	defer func() {
		assert.NotNil(t.T(), recover())
	}()
	t.logger.Panicf("panic")
}

func (t *LoggerTest) Test_Critical() {
	defer func() {
		if v := recover(); v != nil {
			t.T().Fatal(v)
		}
	}()
	t.logger.Critical("critical")
}

func (t *LoggerTest) Test_Criticalf() {
	defer func() {
		if v := recover(); v != nil {
			t.T().Fatal(v)
		}
	}()
	t.logger.Criticalf("critical")
}

func (t *LoggerTest) Test_Warning() {
	defer func() {
		if v := recover(); v != nil {
			t.T().Fatal(v)
		}
	}()
	t.logger.Warning("warning")
}

func (t *LoggerTest) Test_Notice() {
	defer func() {
		if v := recover(); v != nil {
			t.T().Fatal(v)
		}
	}()
	t.logger.Notice("notice")
}

func (t *LoggerTest) Test_Info() {
	defer func() {
		if v := recover(); v != nil {
			t.T().Fatal(v)
		}
	}()
	t.logger.Info("info")
}

func (t *LoggerTest) Test_Debug() {
	defer func() {
		if v := recover(); v != nil {
			t.T().Fatal(v)
		}
	}()
	t.logger.Debug("debug")
}
