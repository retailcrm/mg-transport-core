package testutil

import (
	"bytes"
	"io"
	"sync"
)

type LockableBuffer struct {
	buf bytes.Buffer
	rw  sync.RWMutex
}

func (b *LockableBuffer) Bytes() []byte {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Bytes()
}

func (b *LockableBuffer) AvailableBuffer() []byte {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.AvailableBuffer()
}

func (b *LockableBuffer) String() string {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.String()
}

func (b *LockableBuffer) Len() int {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Len()
}

func (b *LockableBuffer) Cap() int {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Cap()
}

func (b *LockableBuffer) Available() int {
	defer b.rw.RUnlock()
	b.rw.RLock()
	return b.buf.Available()
}

func (b *LockableBuffer) Truncate(n int) {
	defer b.rw.Unlock()
	b.rw.Lock()
	b.buf.Truncate(n)
}

func (b *LockableBuffer) Reset() {
	defer b.rw.Unlock()
	b.rw.Lock()
	b.buf.Reset()
}

func (b *LockableBuffer) Grow(n int) {
	defer b.rw.Unlock()
	b.rw.Lock()
	b.buf.Grow(n)
}

func (b *LockableBuffer) Write(p []byte) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.Write(p)
}

func (b *LockableBuffer) WriteString(s string) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteString(s)
}

func (b *LockableBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadFrom(r)
}

func (b *LockableBuffer) WriteTo(w io.Writer) (n int64, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteTo(w)
}

func (b *LockableBuffer) WriteByte(c byte) error {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteByte(c)
}

func (b *LockableBuffer) WriteRune(r rune) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.WriteRune(r)
}

func (b *LockableBuffer) Read(p []byte) (n int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.Read(p)
}

func (b *LockableBuffer) Next(n int) []byte {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.Next(n)
}

func (b *LockableBuffer) ReadByte() (byte, error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadByte()
}

func (b *LockableBuffer) ReadRune() (r rune, size int, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadRune()
}

func (b *LockableBuffer) UnreadRune() error {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.UnreadRune()
}

func (b *LockableBuffer) UnreadByte() error {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.UnreadByte()
}

func (b *LockableBuffer) ReadBytes(delim byte) (line []byte, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadBytes(delim)
}

func (b *LockableBuffer) ReadString(delim byte) (line string, err error) {
	defer b.rw.Unlock()
	b.rw.Lock()
	return b.buf.ReadString(delim)
}
