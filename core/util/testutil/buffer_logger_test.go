package testutil

import (
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type BufferLoggerTest struct {
	suite.Suite
	logger       BufferedLogger
	silentLogger BufferedLogger
}

func TestBufferLogger(t *testing.T) {
	suite.Run(t, new(BufferLoggerTest))
}

func (t *BufferLoggerTest) SetupSuite() {
	t.logger = NewBufferedLogger()
	t.silentLogger = NewBufferedLoggerSilent()
}

func (t *BufferLoggerTest) SetupTest() {
	t.logger.Reset()
	t.silentLogger.Reset()
}

func (t *BufferLoggerTest) Test_Read() {
	t.logger.Debug("test")

	data, err := io.ReadAll(t.logger)
	t.Require().NoError(err)
	t.Contains(string(data), "\"level_name\":\"DEBUG\"")
	t.Contains(string(data), "\"message\":\"test\"")

	t.silentLogger.Debug("test")

	data, err = io.ReadAll(t.silentLogger)
	t.Require().NoError(err)
	t.Contains(string(data), "\"level_name\":\"DEBUG\"")
	t.Contains(string(data), "\"message\":\"test\"")
}

func (t *BufferLoggerTest) Test_Bytes() {
	t.logger.Debug("test")
	t.Contains(string(t.logger.Bytes()), "\"level_name\":\"DEBUG\"")
	t.Contains(string(t.logger.Bytes()), "\"message\":\"test\"")

	t.silentLogger.Debug("test")
	t.Contains(string(t.silentLogger.Bytes()), "\"level_name\":\"DEBUG\"")
	t.Contains(string(t.silentLogger.Bytes()), "\"message\":\"test\"")
}

func (t *BufferLoggerTest) Test_String() {
	t.logger.Debug("test")
	t.Contains(t.logger.String(), "\"level_name\":\"DEBUG\"")
	t.Contains(t.logger.String(), "\"message\":\"test\"")

	t.silentLogger.Debug("test")
	t.Contains(t.silentLogger.String(), "\"level_name\":\"DEBUG\"")
	t.Contains(t.silentLogger.String(), "\"message\":\"test\"")
}

func (t *BufferLoggerTest) TestRace() {
	var (
		wg      sync.WaitGroup
		starter sync.WaitGroup
	)
	starter.Add(1)
	wg.Add(4)
	go func() {
		starter.Wait()
		t.logger.Debug("test")
		wg.Done()
	}()
	go func() {
		starter.Wait()
		t.logger.String()
		wg.Done()
	}()
	go func() {
		starter.Wait()
		t.silentLogger.Debug("test")
		wg.Done()
	}()
	go func() {
		starter.Wait()
		t.silentLogger.String()
		wg.Done()
	}()
	starter.Done()
	wg.Wait()
}
