package logger

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type NilTest struct {
	suite.Suite
	logger     Logger
	realStdout *os.File
	r          *os.File
	w          *os.File
}

func TestNilLogger(t *testing.T) {
	suite.Run(t, new(NilTest))
}

func (t *NilTest) SetupSuite() {
	t.logger = NewNil()
}

func (t *NilTest) SetupTest() {
	t.realStdout = os.Stdout
	t.r, t.w, _ = os.Pipe()
	os.Stdout = t.w
}

func (t *NilTest) TearDownTest() {
	if t.realStdout != nil {
		t.Require().NoError(t.w.Close())
		os.Stdout = t.realStdout
	}
}

func (t *NilTest) readStdout() string {
	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, t.r)
		t.Require().NoError(err)
		outC <- buf.String()
		close(outC)
	}()

	t.Require().NoError(t.w.Close())
	os.Stdout = t.realStdout
	t.realStdout = nil

	select {
	case c := <-outC:
		return c
	case <-time.After(time.Second):
		return ""
	}
}

func (t *NilTest) Test_Noop() {
	t.logger.Critical("message")
	t.logger.Criticalf("message")
	t.logger.Error("message")
	t.logger.Errorf("message")
	t.logger.Warning("message")
	t.logger.Warningf("message")
	t.logger.Notice("message")
	t.logger.Noticef("message")
	t.logger.Info("message")
	t.logger.Infof("message")
	t.logger.Debug("message")
	t.logger.Debugf("message")

	t.Assert().Empty(t.readStdout())
}

func (t *NilTest) Test_Panic() {
	t.Assert().Panics(func() {
		t.logger.Panic("")
	})
}

func (t *NilTest) Test_Panicf() {
	t.Assert().Panics(func() {
		t.logger.Panicf("")
	})
}
