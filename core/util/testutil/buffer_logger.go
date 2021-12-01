package testutil

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/op/go-logging"

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
	buf bytes.Buffer
}

// NewBufferedLogger returns new BufferedLogger instance.
func NewBufferedLogger() BufferedLogger {
	return &BufferLogger{}
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

func (l *BufferLogger) write(level logging.Level, args ...interface{}) {
	l.buf.WriteString(fmt.Sprintln(append([]interface{}{level.String(), "=>"}, args...)...))
}

func (l *BufferLogger) writef(level logging.Level, format string, args ...interface{}) {
	l.buf.WriteString(fmt.Sprintf(level.String()+" => "+format, args...))
}

func (l *BufferLogger) Fatal(args ...interface{}) {
	l.write(logging.CRITICAL, args...)
	os.Exit(1)
}

func (l *BufferLogger) Fatalf(format string, args ...interface{}) {
	l.writef(logging.CRITICAL, format, args...)
	os.Exit(1)
}

func (l *BufferLogger) Panic(args ...interface{}) {
	l.write(logging.CRITICAL, args...)
	panic(fmt.Sprint(args...))
}

func (l *BufferLogger) Panicf(format string, args ...interface{}) {
	l.writef(logging.CRITICAL, format, args...)
	panic(fmt.Sprintf(format, args...))
}

func (l *BufferLogger) Critical(args ...interface{}) {
	l.write(logging.CRITICAL, args...)
}

func (l *BufferLogger) Criticalf(format string, args ...interface{}) {
	l.writef(logging.CRITICAL, format, args...)
}

func (l *BufferLogger) Error(args ...interface{}) {
	l.write(logging.ERROR, args...)
}

func (l *BufferLogger) Errorf(format string, args ...interface{}) {
	l.writef(logging.ERROR, format, args...)
}

func (l *BufferLogger) Warning(args ...interface{}) {
	l.write(logging.WARNING, args...)
}

func (l *BufferLogger) Warningf(format string, args ...interface{}) {
	l.writef(logging.WARNING, format, args...)
}

func (l *BufferLogger) Notice(args ...interface{}) {
	l.write(logging.NOTICE, args...)
}

func (l *BufferLogger) Noticef(format string, args ...interface{}) {
	l.writef(logging.NOTICE, format, args...)
}

func (l *BufferLogger) Info(args ...interface{}) {
	l.write(logging.INFO, args...)
}

func (l *BufferLogger) Infof(format string, args ...interface{}) {
	l.writef(logging.INFO, format, args...)
}

func (l *BufferLogger) Debug(args ...interface{}) {
	l.write(logging.DEBUG, args...)
}

func (l *BufferLogger) Debugf(format string, args ...interface{}) {
	l.writef(logging.DEBUG, format, args...)
}
