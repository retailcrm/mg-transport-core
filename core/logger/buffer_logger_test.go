package logger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/guregu/null/v5"
	"io"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logRecord struct {
	LevelName  string                 `json:"level_name"`
	DateTime   null.Time              `json:"datetime"`
	Caller     string                 `json:"caller"`
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

func (l *bufferLogger) ForHandler(handler any) Logger {
	return l.WithLazy(zap.Any(HandlerAttr, handler))
}

func (l *bufferLogger) ForConnection(conn any) Logger {
	return l.WithLazy(zap.Any(ConnectionAttr, conn))
}

func (l *bufferLogger) ForAccount(acc any) Logger {
	return l.WithLazy(zap.Any(AccountAttr, acc))
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
