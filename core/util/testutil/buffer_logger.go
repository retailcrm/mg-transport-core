package testutil

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
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
	bl.Logger = slog.New(slog.NewTextHandler(&bl.buf, logger.DefaultOpts))
	return bl
}

// With doesn't do anything here and only added for backwards compatibility with the interface.
func (l *BufferLogger) With(args ...any) logger.Logger {
	return &BufferLogger{
		Default: logger.Default{
			Logger: l.Logger.With(args...),
		},
	}
}

func (l *BufferLogger) ForAccount(handler, conn, acc any) logger.Logger {
	return l.With(slog.Any(logger.HandlerAttr, handler), slog.Any(logger.ConnectionAttr, conn), slog.Any(logger.AccountAttr, acc))
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
