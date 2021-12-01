package testutil

import (
	"io"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/suite"
)

type BufferLoggerTest struct {
	suite.Suite
	logger BufferedLogger
}

func TestBufferLogger(t *testing.T) {
	suite.Run(t, new(BufferLoggerTest))
}

func (t *BufferLoggerTest) SetupSuite() {
	t.logger = NewBufferedLogger()
}

func (t *BufferLoggerTest) SetupTest() {
	t.logger.Reset()
}

func (t *BufferLoggerTest) Log() string {
	return t.logger.String()
}

func (t *BufferLoggerTest) Test_Read() {
	t.logger.Debug("test")

	data, err := io.ReadAll(t.logger)
	t.Require().NoError(err)
	t.Assert().Equal([]byte(logging.DEBUG.String()+" => test\n"), data)
}

func (t *BufferLoggerTest) Test_Bytes() {
	t.logger.Debug("test")
	t.Assert().Equal([]byte(logging.DEBUG.String()+" => test\n"), t.logger.Bytes())
}
