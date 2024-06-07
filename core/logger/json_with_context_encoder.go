// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package logger

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"math"
	"time"
	"unicode/utf8"

	"go.uber.org/zap/buffer"
)

// For JSON-escaping; see jsonWithContextEncoder.safeAddString below.
const _hex = "0123456789abcdef"

var _jsonWithContextPool = NewPool(func() *jsonWithContextEncoder {
	return &jsonWithContextEncoder{}
})

func init() {
	err := zap.RegisterEncoder("json-with-context", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return NewJSONWithContextEncoder(config), nil
	})
	if err != nil {
		panic(err)
	}
}

func putJSONWithContextEncoder(enc *jsonWithContextEncoder) {
	if enc.reflectBuf != nil {
		enc.reflectBuf.Free()
	}
	enc.EncoderConfig = nil
	enc.buf = nil
	enc.spaced = false
	enc.openNamespaces = 0
	enc.reflectBuf = nil
	enc.reflectEnc = nil
	_jsonWithContextPool.Put(enc)
}

type jsonWithContextEncoder struct {
	*zapcore.EncoderConfig
	buf            *buffer.Buffer
	spaced         bool // include spaces after colons and commas
	openNamespaces int

	// for encoding generic values by reflection
	reflectBuf *buffer.Buffer
	reflectEnc zapcore.ReflectedEncoder
}

// NewJSONWithContextEncoder creates a fast, low-allocation JSON encoder. The encoder
// appropriately escapes all field keys and values.
//
// Note that the encoder doesn't deduplicate keys, so it's possible to produce
// a message like
//
//	{"foo":"bar","foo":"baz"}
//
// This is permitted by the JSON specification, but not encouraged. Many
// libraries will ignore duplicate key-value pairs (typically keeping the last
// pair) when unmarshaling, but users should attempt to avoid adding duplicate
// keys.
func NewJSONWithContextEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return newJSONWithContextEncoder(cfg, false)
}

func newJSONWithContextEncoder(cfg zapcore.EncoderConfig, spaced bool) *jsonWithContextEncoder {
	if cfg.SkipLineEnding {
		cfg.LineEnding = ""
	} else if cfg.LineEnding == "" {
		cfg.LineEnding = zapcore.DefaultLineEnding
	}

	// If no EncoderConfig.NewReflectedEncoder is provided by the user, then use default
	if cfg.NewReflectedEncoder == nil {
		cfg.NewReflectedEncoder = defaultReflectedEncoder
	}

	return &jsonWithContextEncoder{
		EncoderConfig: &cfg,
		buf:           GetBufferPool(),
		spaced:        spaced,
	}
}

func defaultReflectedEncoder(w io.Writer) zapcore.ReflectedEncoder {
	enc := json.NewEncoder(w)
	// For consistency with our custom JSON encoder.
	enc.SetEscapeHTML(false)
	return enc
}

func (enc *jsonWithContextEncoder) AddArray(key string, arr zapcore.ArrayMarshaler) error {
	enc.addKey(key)
	return enc.AppendArray(arr)
}

func (enc *jsonWithContextEncoder) AddObject(key string, obj zapcore.ObjectMarshaler) error {
	enc.addKey(key)
	return enc.AppendObject(obj)
}

func (enc *jsonWithContextEncoder) AddBinary(key string, val []byte) {
	enc.AddString(key, base64.StdEncoding.EncodeToString(val))
}

func (enc *jsonWithContextEncoder) AddByteString(key string, val []byte) {
	enc.addKey(key)
	enc.AppendByteString(val)
}

func (enc *jsonWithContextEncoder) AddBool(key string, val bool) {
	enc.addKey(key)
	enc.AppendBool(val)
}

func (enc *jsonWithContextEncoder) AddComplex128(key string, val complex128) {
	enc.addKey(key)
	enc.AppendComplex128(val)
}

func (enc *jsonWithContextEncoder) AddComplex64(key string, val complex64) {
	enc.addKey(key)
	enc.AppendComplex64(val)
}

func (enc *jsonWithContextEncoder) AddDuration(key string, val time.Duration) {
	enc.addKey(key)
	enc.AppendDuration(val)
}

func (enc *jsonWithContextEncoder) AddFloat64(key string, val float64) {
	enc.addKey(key)
	enc.AppendFloat64(val)
}

func (enc *jsonWithContextEncoder) AddFloat32(key string, val float32) {
	enc.addKey(key)
	enc.AppendFloat32(val)
}

func (enc *jsonWithContextEncoder) AddInt64(key string, val int64) {
	enc.addKey(key)
	enc.AppendInt64(val)
}

func (enc *jsonWithContextEncoder) resetReflectBuf() {
	if enc.reflectBuf == nil {
		enc.reflectBuf = GetBufferPool()
		enc.reflectEnc = enc.NewReflectedEncoder(enc.reflectBuf)
	} else {
		enc.reflectBuf.Reset()
	}
}

var nullLiteralBytes = []byte("null")

// Only invoke the standard JSON encoder if there is actually something to
// encode; otherwise write JSON null literal directly.
func (enc *jsonWithContextEncoder) encodeReflected(obj interface{}) ([]byte, error) {
	if obj == nil {
		return nullLiteralBytes, nil
	}
	enc.resetReflectBuf()
	if err := enc.reflectEnc.Encode(obj); err != nil {
		return nil, err
	}
	enc.reflectBuf.TrimNewline()
	return enc.reflectBuf.Bytes(), nil
}

func (enc *jsonWithContextEncoder) AddReflected(key string, obj interface{}) error {
	valueBytes, err := enc.encodeReflected(obj)
	if err != nil {
		return err
	}
	enc.addKey(key)
	_, err = enc.buf.Write(valueBytes)
	return err
}

func (enc *jsonWithContextEncoder) OpenNamespace(key string) {
	enc.addKey(key)
	enc.buf.AppendByte('{')
	enc.openNamespaces++
}

func (enc *jsonWithContextEncoder) AddString(key, val string) {
	enc.addKey(key)
	enc.AppendString(val)
}

func (enc *jsonWithContextEncoder) AddTime(key string, val time.Time) {
	enc.addKey(key)
	enc.AppendTime(val)
}

func (enc *jsonWithContextEncoder) AddUint64(key string, val uint64) {
	enc.addKey(key)
	enc.AppendUint64(val)
}

func (enc *jsonWithContextEncoder) AppendArray(arr zapcore.ArrayMarshaler) error {
	enc.addElementSeparator()
	enc.buf.AppendByte('[')
	err := arr.MarshalLogArray(enc)
	enc.buf.AppendByte(']')
	return err
}

func (enc *jsonWithContextEncoder) AppendObject(obj zapcore.ObjectMarshaler) error {
	// Close ONLY new openNamespaces that are created during
	// AppendObject().
	old := enc.openNamespaces
	enc.openNamespaces = 0
	enc.addElementSeparator()
	enc.buf.AppendByte('{')
	err := obj.MarshalLogObject(enc)
	enc.buf.AppendByte('}')
	enc.closeOpenNamespaces()
	enc.openNamespaces = old
	return err
}

func (enc *jsonWithContextEncoder) AppendBool(val bool) {
	enc.addElementSeparator()
	enc.buf.AppendBool(val)
}

func (enc *jsonWithContextEncoder) AppendByteString(val []byte) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddByteString(val)
	enc.buf.AppendByte('"')
}

// appendComplex appends the encoded form of the provided complex128 value.
// precision specifies the encoding precision for the real and imaginary
// components of the complex number.
func (enc *jsonWithContextEncoder) appendComplex(val complex128, precision int) {
	enc.addElementSeparator()
	// Cast to a platform-independent, fixed-size type.
	r, i := float64(real(val)), float64(imag(val))
	enc.buf.AppendByte('"')
	// Because we're always in a quoted string, we can use strconv without
	// special-casing NaN and +/-Inf.
	enc.buf.AppendFloat(r, precision)
	// If imaginary part is less than 0, minus (-) sign is added by default
	// by AppendFloat.
	if i >= 0 {
		enc.buf.AppendByte('+')
	}
	enc.buf.AppendFloat(i, precision)
	enc.buf.AppendByte('i')
	enc.buf.AppendByte('"')
}

func (enc *jsonWithContextEncoder) AppendDuration(val time.Duration) {
	cur := enc.buf.Len()
	if e := enc.EncodeDuration; e != nil {
		e(val, enc)
	}
	if cur == enc.buf.Len() {
		// User-supplied EncodeDuration is a no-op. Fall back to nanoseconds to keep
		// JSON valid.
		enc.AppendInt64(int64(val))
	}
}

func (enc *jsonWithContextEncoder) AppendInt64(val int64) {
	enc.addElementSeparator()
	enc.buf.AppendInt(val)
}

func (enc *jsonWithContextEncoder) AppendReflected(val interface{}) error {
	valueBytes, err := enc.encodeReflected(val)
	if err != nil {
		return err
	}
	enc.addElementSeparator()
	_, err = enc.buf.Write(valueBytes)
	return err
}

func (enc *jsonWithContextEncoder) AppendString(val string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(val)
	enc.buf.AppendByte('"')
}

func (enc *jsonWithContextEncoder) AppendTimeLayout(time time.Time, layout string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.buf.AppendTime(time, layout)
	enc.buf.AppendByte('"')
}

func (enc *jsonWithContextEncoder) AppendTime(val time.Time) {
	cur := enc.buf.Len()
	if e := enc.EncodeTime; e != nil {
		e(val, enc)
	}
	if cur == enc.buf.Len() {
		// User-supplied EncodeTime is a no-op. Fall back to nanos since epoch to keep
		// output JSON valid.
		enc.AppendInt64(val.UnixNano())
	}
}

func (enc *jsonWithContextEncoder) AppendUint64(val uint64) {
	enc.addElementSeparator()
	enc.buf.AppendUint(val)
}

func (enc *jsonWithContextEncoder) AddInt(k string, v int)         { enc.AddInt64(k, int64(v)) }
func (enc *jsonWithContextEncoder) AddInt32(k string, v int32)     { enc.AddInt64(k, int64(v)) }
func (enc *jsonWithContextEncoder) AddInt16(k string, v int16)     { enc.AddInt64(k, int64(v)) }
func (enc *jsonWithContextEncoder) AddInt8(k string, v int8)       { enc.AddInt64(k, int64(v)) }
func (enc *jsonWithContextEncoder) AddUint(k string, v uint)       { enc.AddUint64(k, uint64(v)) }
func (enc *jsonWithContextEncoder) AddUint32(k string, v uint32)   { enc.AddUint64(k, uint64(v)) }
func (enc *jsonWithContextEncoder) AddUint16(k string, v uint16)   { enc.AddUint64(k, uint64(v)) }
func (enc *jsonWithContextEncoder) AddUint8(k string, v uint8)     { enc.AddUint64(k, uint64(v)) }
func (enc *jsonWithContextEncoder) AddUintptr(k string, v uintptr) { enc.AddUint64(k, uint64(v)) }
func (enc *jsonWithContextEncoder) AppendComplex64(v complex64)    { enc.appendComplex(complex128(v), 32) }
func (enc *jsonWithContextEncoder) AppendComplex128(v complex128) {
	enc.appendComplex(complex128(v), 64)
}
func (enc *jsonWithContextEncoder) AppendFloat64(v float64) { enc.appendFloat(v, 64) }
func (enc *jsonWithContextEncoder) AppendFloat32(v float32) { enc.appendFloat(float64(v), 32) }
func (enc *jsonWithContextEncoder) AppendInt(v int)         { enc.AppendInt64(int64(v)) }
func (enc *jsonWithContextEncoder) AppendInt32(v int32)     { enc.AppendInt64(int64(v)) }
func (enc *jsonWithContextEncoder) AppendInt16(v int16)     { enc.AppendInt64(int64(v)) }
func (enc *jsonWithContextEncoder) AppendInt8(v int8)       { enc.AppendInt64(int64(v)) }
func (enc *jsonWithContextEncoder) AppendUint(v uint)       { enc.AppendUint64(uint64(v)) }
func (enc *jsonWithContextEncoder) AppendUint32(v uint32)   { enc.AppendUint64(uint64(v)) }
func (enc *jsonWithContextEncoder) AppendUint16(v uint16)   { enc.AppendUint64(uint64(v)) }
func (enc *jsonWithContextEncoder) AppendUint8(v uint8)     { enc.AppendUint64(uint64(v)) }
func (enc *jsonWithContextEncoder) AppendUintptr(v uintptr) { enc.AppendUint64(uint64(v)) }

func (enc *jsonWithContextEncoder) Clone() zapcore.Encoder {
	clone := enc.clone()
	clone.buf.Write(enc.buf.Bytes())
	return clone
}

func (enc *jsonWithContextEncoder) clone() *jsonWithContextEncoder {
	clone := _jsonWithContextPool.Get()
	clone.EncoderConfig = enc.EncoderConfig
	clone.spaced = enc.spaced
	clone.openNamespaces = enc.openNamespaces
	clone.buf = GetBufferPool()
	return clone
}

func (enc *jsonWithContextEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := enc.clone()
	final.buf.AppendByte('{')

	if final.LevelKey != "" && final.EncodeLevel != nil {
		final.addKey(final.LevelKey)
		cur := final.buf.Len()
		final.EncodeLevel(ent.Level, final)
		if cur == final.buf.Len() {
			// User-supplied EncodeLevel was a no-op. Fall back to strings to keep
			// output JSON valid.
			final.AppendString(ent.Level.String())
		}
	}
	if final.TimeKey != "" {
		final.AddTime(final.TimeKey, ent.Time)
	}
	if ent.LoggerName != "" && final.NameKey != "" {
		final.addKey(final.NameKey)
		cur := final.buf.Len()
		nameEncoder := final.EncodeName

		// if no name encoder provided, fall back to FullNameEncoder for backwards
		// compatibility
		if nameEncoder == nil {
			nameEncoder = zapcore.FullNameEncoder
		}

		nameEncoder(ent.LoggerName, final)
		if cur == final.buf.Len() {
			// User-supplied EncodeName was a no-op. Fall back to strings to
			// keep output JSON valid.
			final.AppendString(ent.LoggerName)
		}
	}
	if ent.Caller.Defined {
		if final.CallerKey != "" {
			final.addKey(final.CallerKey)
			cur := final.buf.Len()
			final.EncodeCaller(ent.Caller, final)
			if cur == final.buf.Len() {
				// User-supplied EncodeCaller was a no-op. Fall back to strings to
				// keep output JSON valid.
				final.AppendString(ent.Caller.String())
			}
		}
		if final.FunctionKey != "" {
			final.addKey(final.FunctionKey)
			final.AppendString(ent.Caller.Function)
		}
	}
	if final.MessageKey != "" {
		final.addKey(enc.MessageKey)
		final.AppendString(ent.Message)
	}
	if enc.buf.Len() > 0 {
		final.addElementSeparator()
		final.buf.Write(enc.buf.Bytes())
	}
	addFields(final, fields)
	final.closeOpenNamespaces()
	if ent.Stack != "" && final.StacktraceKey != "" {
		final.AddString(final.StacktraceKey, ent.Stack)
	}
	final.buf.AppendByte('}')
	final.buf.AppendString(final.LineEnding)

	ret := final.buf
	putJSONWithContextEncoder(final)
	return ret, nil
}

func addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	m := make(map[string]interface{})
	hasEntries := false
	for _, f := range fields {
		switch f.Key {
		case HandlerAttr, ConnectionAttr, AccountAttr:
			f.AddTo(enc)
		default:
			hasEntries = true
			if f.Interface != nil {
				switch t := f.Interface.(type) {
				case fmt.Stringer:
					m[f.Key] = t.String()
				case fmt.GoStringer:
					m[f.Key] = t.GoString()
				case error:
					m[f.Key] = t.Error()
				default:
					m[f.Key] = f.Interface
				}
				continue
			}
			if f.String != "" {
				m[f.Key] = f.String
				continue
			}
			m[f.Key] = f.Integer
		}
	}
	if hasEntries {
		zap.Any("context", m).AddTo(enc)
	}
}

func (enc *jsonWithContextEncoder) truncate() {
	enc.buf.Reset()
}

func (enc *jsonWithContextEncoder) closeOpenNamespaces() {
	for i := 0; i < enc.openNamespaces; i++ {
		enc.buf.AppendByte('}')
	}
	enc.openNamespaces = 0
}

func (enc *jsonWithContextEncoder) addKey(key string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(key)
	enc.buf.AppendByte('"')
	enc.buf.AppendByte(':')
	if enc.spaced {
		enc.buf.AppendByte(' ')
	}
}

func (enc *jsonWithContextEncoder) addElementSeparator() {
	last := enc.buf.Len() - 1
	if last < 0 {
		return
	}
	switch enc.buf.Bytes()[last] {
	case '{', '[', ':', ',', ' ':
		return
	default:
		enc.buf.AppendByte(',')
		if enc.spaced {
			enc.buf.AppendByte(' ')
		}
	}
}

func (enc *jsonWithContextEncoder) appendFloat(val float64, bitSize int) {
	enc.addElementSeparator()
	switch {
	case math.IsNaN(val):
		enc.buf.AppendString(`"NaN"`)
	case math.IsInf(val, 1):
		enc.buf.AppendString(`"+Inf"`)
	case math.IsInf(val, -1):
		enc.buf.AppendString(`"-Inf"`)
	default:
		enc.buf.AppendFloat(val, bitSize)
	}
}

// safeAddString JSON-escapes a string and appends it to the internal buffer.
// Unlike the standard library's encoder, it doesn't attempt to protect the
// user from browser vulnerabilities or JSONP-related problems.
func (enc *jsonWithContextEncoder) safeAddString(s string) {
	safeAppendStringLike(
		(*buffer.Buffer).AppendString,
		utf8.DecodeRuneInString,
		enc.buf,
		s,
	)
}

// safeAddByteString is no-alloc equivalent of safeAddString(string(s)) for s []byte.
func (enc *jsonWithContextEncoder) safeAddByteString(s []byte) {
	safeAppendStringLike(
		(*buffer.Buffer).AppendBytes,
		utf8.DecodeRune,
		enc.buf,
		s,
	)
}

// safeAppendStringLike is a generic implementation of safeAddString and safeAddByteString.
// It appends a string or byte slice to the buffer, escaping all special characters.
func safeAppendStringLike[S []byte | string](
	// appendTo appends this string-like object to the buffer.
	appendTo func(*buffer.Buffer, S),
	// decodeRune decodes the next rune from the string-like object
	// and returns its value and width in bytes.
	decodeRune func(S) (rune, int),
	buf *buffer.Buffer,
	s S,
) {
	// The encoding logic below works by skipping over characters
	// that can be safely copied as-is,
	// until a character is found that needs special handling.
	// At that point, we copy everything we've seen so far,
	// and then handle that special character.
	//
	// last is the index of the last byte that was copied to the buffer.
	last := 0
	for i := 0; i < len(s); {
		if s[i] >= utf8.RuneSelf {
			// Character >= RuneSelf may be part of a multi-byte rune.
			// They need to be decoded before we can decide how to handle them.
			r, size := decodeRune(s[i:])
			if r != utf8.RuneError || size != 1 {
				// No special handling required.
				// Skip over this rune and continue.
				i += size
				continue
			}

			// Invalid UTF-8 sequence.
			// Replace it with the Unicode replacement character.
			appendTo(buf, s[last:i])
			buf.AppendString(`\ufffd`)

			i++
			last = i
		} else {
			// Character < RuneSelf is a single-byte UTF-8 rune.
			if s[i] >= 0x20 && s[i] != '\\' && s[i] != '"' {
				// No escaping necessary.
				// Skip over this character and continue.
				i++
				continue
			}

			// This character needs to be escaped.
			appendTo(buf, s[last:i])
			switch s[i] {
			case '\\', '"':
				buf.AppendByte('\\')
				buf.AppendByte(s[i])
			case '\n':
				buf.AppendByte('\\')
				buf.AppendByte('n')
			case '\r':
				buf.AppendByte('\\')
				buf.AppendByte('r')
			case '\t':
				buf.AppendByte('\\')
				buf.AppendByte('t')
			default:
				// Encode bytes < 0x20, except for the escape sequences above.
				buf.AppendString(`\u00`)
				buf.AppendByte(_hex[s[i]>>4])
				buf.AppendByte(_hex[s[i]&0xF])
			}

			i++
			last = i
		}
	}

	// add remaining
	appendTo(buf, s[last:])
}
