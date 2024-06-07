package testutil

import (
	"fmt"
	"io"
	"os"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ReadBuffer is implemented by the BufferLogger.
// Its methods give access to the buffer contents and ability to read buffer as an io.Reader or reset its contents.
type ReadBuffer interface {
	io.Reader
	fmt.Stringer
	Bytes() []byte
	Reset()
}

// BufferedLogger is a logger that can return the data written to it.
type BufferedLogger interface {
	ReadBuffer
	logger.Logger
}

// BufferLogger is an implementation of the BufferedLogger.
type BufferLogger struct {
	logger.Default
	buf LockableBuffer
}

// NewBufferedLogger returns new BufferedLogger instance.
func NewBufferedLogger() BufferedLogger {
	bl := &BufferLogger{}
	bl.Logger = zap.New(
		zapcore.NewCore(
			logger.NewJSONWithContextEncoder(
				logger.EncoderConfigJSON()), zap.CombineWriteSyncers(os.Stdout, os.Stderr, &bl.buf), zapcore.DebugLevel))
	return bl
}

func (l *BufferLogger) With(fields ...zapcore.Field) logger.Logger {
	return &BufferLogger{
		Default: logger.Default{
			Logger: l.Logger.With(fields...),
		},
	}
}

func (l *BufferLogger) WithLazy(fields ...zapcore.Field) logger.Logger {
	return &BufferLogger{
		Default: logger.Default{
			Logger: l.Logger.WithLazy(fields...),
		},
	}
}

func (l *BufferLogger) ForHandler(handler any) logger.Logger {
	return l.WithLazy(zap.Any(logger.HandlerAttr, handler))
}

func (l *BufferLogger) ForConnection(conn any) logger.Logger {
	return l.WithLazy(zap.Any(logger.ConnectionAttr, conn))
}

func (l *BufferLogger) ForAccount(acc any) logger.Logger {
	return l.WithLazy(zap.Any(logger.AccountAttr, acc))
}

// Read bytes from the logger buffer. io.Reader implementation.
func (l *BufferLogger) Read(p []byte) (n int, err error) {
	return l.buf.Read(p)
}

// String contents of the logger buffer. fmt.Stringer implementation.
func (l *BufferLogger) String() string {
	return l.buf.String()
}

// Bytes is a shorthand for the underlying bytes.Buffer method. Returns byte slice with the buffer contents.
func (l *BufferLogger) Bytes() []byte {
	return l.buf.Bytes()
}

// Reset is a shorthand for the underlying bytes.Buffer method. It will reset buffer contents.
func (l *BufferLogger) Reset() {
	l.buf.Reset()
}
