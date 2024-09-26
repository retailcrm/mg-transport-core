package logger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/guregu/null/v5"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logRecord struct {
	LevelName  string                 `json:"level_name"`
	DateTime   null.Time              `json:"datetime"`
	StreamID   string                 `json:"streamId"`
	Message    string                 `json:"message"`
	Handler    string                 `json:"handler,omitempty"`
	Connection string                 `json:"connection,omitempty"`
	Account    string                 `json:"account,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

type jSONRecordScanner struct {
	scan *bufio.Scanner
	buf  *bufferLogger
}

func newJSONBufferedLogger(buf *bufferLogger) *jSONRecordScanner {
	if buf == nil {
		buf = newBufferLogger()
	}
	return &jSONRecordScanner{scan: bufio.NewScanner(buf), buf: buf}
}

func (s *jSONRecordScanner) ScanAll() ([]logRecord, error) {
	var entries []logRecord
	for s.scan.Scan() {
		entry := logRecord{}
		if err := json.Unmarshal(s.scan.Bytes(), &entry); err != nil {
			return entries, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *jSONRecordScanner) Logger() Logger {
	return s.buf
}

// bufferLogger is an implementation of the BufferedLogger.
type bufferLogger struct {
	Default
	buf lockableBuffer
}

// NewBufferedLogger returns new BufferedLogger instance.
func newBufferLogger() *bufferLogger {
	bl := &bufferLogger{}
	bl.Logger = zap.New(
		zapcore.NewCore(
			NewJSONWithContextEncoder(
				EncoderConfigJSON()), zap.CombineWriteSyncers(os.Stdout, os.Stderr, &bl.buf), zapcore.DebugLevel))
	return bl
}

func (l *bufferLogger) With(fields ...zapcore.Field) Logger {
	return &bufferLogger{
		Default: Default{
			Logger: l.Logger.With(fields...),
		},
	}
}

func (l *bufferLogger) WithLazy(fields ...zapcore.Field) Logger {
	return &bufferLogger{
		Default: Default{
			Logger: l.Logger.WithLazy(fields...),
		},
	}
}

// ForHandler returns a new logger that is associated with the given handler.
// This will replace "handler" field if it was set before.
// Note: chain calls like ForHandler().With().ForHandler() will DUPLICATE handler field!
func (l *bufferLogger) ForHandler(handler any) Logger {
	if l.previous != previousFieldHandler {
		result := l.With(zap.Any(HandlerAttr, handler))
		result.(*bufferLogger).setPrevious(previousFieldHandler)
		result.(*bufferLogger).parent = l.Logger
		return result
	}
	result := l.clone(l.parentOrCurrent().With(zap.Any(HandlerAttr, handler)))
	result.(*bufferLogger).setPrevious(previousFieldHandler)
	return result
}

// ForConnection returns a new logger that is associated with the given connection.
// This will replace "connection" field if it was set before.
// Note: chain calls like ForConnection().With().ForConnection() will DUPLICATE connection field!
func (l *bufferLogger) ForConnection(conn any) Logger {
	if l.previous != previousFieldConnection {
		result := l.With(zap.Any(ConnectionAttr, conn))
		result.(*bufferLogger).setPrevious(previousFieldConnection)
		result.(*bufferLogger).parent = l.Logger
		return result
	}
	result := l.clone(l.parentOrCurrent().With(zap.Any(ConnectionAttr, conn)))
	result.(*bufferLogger).setPrevious(previousFieldConnection)
	return result
}

// ForAccount returns a new logger that is associated with the given account.
// This will replace "account" field if it was set before.
// Note: chain calls like ForAccount().With().ForAccount() will DUPLICATE account field!
func (l *bufferLogger) ForAccount(acc any) Logger {
	if l.previous != previousFieldAccount {
		result := l.With(zap.Any(AccountAttr, acc))
		result.(*bufferLogger).setPrevious(previousFieldAccount)
		result.(*bufferLogger).parent = l.Logger
		return result
	}
	result := l.clone(l.parentOrCurrent().With(zap.Any(AccountAttr, acc)))
	result.(*bufferLogger).setPrevious(previousFieldAccount)
	return result
}

// Read bytes from the logger buffer. io.Reader implementation.
func (l *bufferLogger) Read(p []byte) (n int, err error) {
	return l.buf.Read(p)
}

// String contents of the logger buffer. fmt.Stringer implementation.
func (l *bufferLogger) String() string {
	return l.buf.String()
}

// Bytes is a shorthand for the underlying bytes.Buffer method. Returns byte slice with the buffer contents.
func (l *bufferLogger) Bytes() []byte {
	return l.buf.Bytes()
}

// Reset is a shorthand for the underlying bytes.Buffer method. It will reset buffer contents.
func (l *bufferLogger) Reset() {
	l.buf.Reset()
}

// clone creates a copy of the given logger.
func (l *bufferLogger) clone(log *zap.Logger) Logger {
	parent := l.parent
	if parent == nil {
		parent = l.Logger
	}
	return &bufferLogger{
		Default: Default{
			Logger: log,
			parent: parent,
		},
	}
}

// parentOrCurrent returns parent logger if it exists or current logger otherwise.
func (l *bufferLogger) parentOrCurrent() *zap.Logger {
	if l.parent != nil {
		return l.parent
	}
	return l.Logger
}

type lockableBuffer struct {
	buf bytes.Buffer
	rw  sync.RWMutex
}

func (b *lockableBuffer) Bytes() []byte {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Bytes()
}

func (b *lockableBuffer) AvailableBuffer() []byte {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.AvailableBuffer()
}

func (b *lockableBuffer) String() string {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.String()
}

func (b *lockableBuffer) Len() int {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Len()
}

func (b *lockableBuffer) Cap() int {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Cap()
}

func (b *lockableBuffer) Available() int {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Available()
}

func (b *lockableBuffer) Truncate(n int) {
	defer b.rw.Unlock()
	b.rw.Lock()
	b.buf.Truncate(n)
}

func (b *lockableBuffer) Reset() {
	defer b.rw.Unlock()
	b.rw.Lock()
	b.buf.Reset()
}

func (b *lockableBuffer) Grow(n int) {
	defer b.rw.Unlock()
	b.rw.Lock()
	b.buf.Grow(n)
}

func (b *lockableBuffer) Write(p []byte) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.Write(p)
}

func (b *lockableBuffer) WriteString(s string) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteString(s)
}

func (b *lockableBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadFrom(r)
}

func (b *lockableBuffer) WriteTo(w io.Writer) (n int64, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteTo(w)
}

func (b *lockableBuffer) WriteByte(c byte) error {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteByte(c)
}

func (b *lockableBuffer) WriteRune(r rune) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteRune(r)
}

func (b *lockableBuffer) Read(p []byte) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.Read(p)
}

func (b *lockableBuffer) Next(n int) []byte {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.Next(n)
}

func (b *lockableBuffer) ReadByte() (byte, error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadByte()
}

func (b *lockableBuffer) ReadRune() (r rune, size int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadRune()
}

func (b *lockableBuffer) UnreadRune() error {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.UnreadRune()
}

func (b *lockableBuffer) UnreadByte() error {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.UnreadByte()
}

func (b *lockableBuffer) ReadBytes(delim byte) (line []byte, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadBytes(delim)
}

func (b *lockableBuffer) ReadString(delim byte) (line string, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadString(delim)
}

// Sync is a no-op.
func (b *lockableBuffer) Sync() error {
	return nil
}
