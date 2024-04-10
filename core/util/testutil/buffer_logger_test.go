package testutil

import (
	"io"
	"testing"

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

func (t *BufferLoggerTest) Test_Read() {
	t.logger.Debug("test")

	data, err := io.ReadAll(t.logger)
	t.Require().NoError(err)
	t.Assert().Contains(string(data), "level=DEBUG test")
}

func (t *BufferLoggerTest) Test_Bytes() {
	t.logger.Debug("test")
	t.Assert().Contains(string(t.logger.Bytes()), "level=DEBUG test")
}

func (t *BufferLoggerTest) Test_String() {
	t.logger.Debug("test")
	t.Assert().Contains(t.logger.String(), "level=DEBUG test")
}

func (t *BufferLoggerTest) TestRace() {
	go func() {
		t.logger.Debug("test")
	}()
	go func() {
		t.logger.String()
	}()
}
