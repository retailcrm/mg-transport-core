package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newMaskTestLogger(format string) (*Default, *lockableBuffer) {
	var enc zapcore.Encoder

	if format == "json" {
		cfg := EncoderConfigJSON()
		cfg.TimeKey, cfg.LevelKey = "", ""
		enc = NewJSONWithContextEncoder(cfg)
	} else {
		cfg := EncoderConfigConsole()
		cfg.TimeKey, cfg.LevelKey = "", ""
		enc = newMaskingConsoleEncoder(cfg)
	}

	buf := &lockableBuffer{}

	return &Default{Logger: zap.New(zapcore.NewCore(enc, buf, zapcore.DebugLevel))}, buf
}

func maskTestRecords(t *testing.T, format, output string) []logRecord {
	t.Helper()

	var records []logRecord

	for line := range strings.SplitSeq(strings.TrimSuffix(output, "\n"), "\n") {
		var record logRecord

		if format == "json" {
			require.NoError(t, json.Unmarshal([]byte(line), &record))

			// With fields are encoded at the root; call-site fields live in context.
			var fields map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &fields))
			delete(fields, "message")
			delete(fields, "context")

			for key, value := range record.Context {
				fields[key] = value
			}

			record.Context = fields
		} else {
			message, context, hasContext := strings.Cut(line, " {")
			record.Message = message

			if hasContext {
				require.NoError(t, json.Unmarshal([]byte("{"+context), &record.Context))
			}
		}

		records = append(records, record)
	}

	return records
}

func TestMaskScope(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		for _, lazy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/lazy=%t", format, lazy), func(t *testing.T) {
				root, buf := newMaskTestLogger(format)
				parent := root.With(zap.String("inherited", "alpha beta gamma"))
				with := parent.With

				if lazy {
					with = parent.WithLazy
				}

				child := with(Mask("alpha", "beta"))
				sibling := with(Mask("gamma"))
				grandchild := child.With(Mask("gamma"))

				parent.Info("alpha beta gamma")
				child.Info("alpha beta gamma", zap.String("field", "alpha beta gamma"))
				sibling.Info("alpha beta gamma")
				grandchild.Info("alpha beta gamma")
				child.Info("alpha beta gamma")

				parent.Info("alpha beta gamma")

				records := maskTestRecords(t, format, buf.String())
				require.Len(t, records, 6)

				for i, want := range []string{
					"alpha beta gamma", "**** **** gamma", "alpha beta ****",
					"**** **** ****", "**** **** gamma", "alpha beta gamma",
				} {
					assert.Equal(t, want, records[i].Message)
					assert.Equal(t, want, records[i].Context["inherited"])
					assert.NotContains(t, records[i].Context, "")
				}

				assert.Equal(t, "**** **** gamma", records[1].Context["field"])
			})
		}
	}
}

type maskTestJSON string

func (value maskTestJSON) MarshalJSON() ([]byte, error) {
	return []byte(value), nil
}

func TestMaskStructuredValues(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			root, buf := newMaskTestLogger(format)
			secret := "line\n\"quoted\"\\token☺<>&"
			body, err := json.Marshal(map[string]any{"nested": []any{secret, map[string]any{"key": "Bearer secret"}}})
			require.NoError(t, err)

			log := root.With(zap.Any("inherited", map[string]any{"secret": secret}),
				zap.ByteString("bytes", []byte(secret))).
				With(Mask(secret, "secret"))

			log.Info("request "+secret,
				Body(body),
				zap.Error(errors.New("failed: "+secret)),
				zap.Stringer("stringer", maskTestStringer(secret)),
				zap.Reflect("escaped", maskTestJSON(`{"value":"sec\u0072et"}`)))

			records := maskTestRecords(t, format, buf.String())
			require.Len(t, records, 1)
			assert.Equal(t, "request ****", records[0].Message)
			assert.Equal(t, map[string]any{"secret": "****"}, records[0].Context["inherited"])
			assert.Equal(t, "failed: ****", records[0].Context["error"])
			assert.Equal(t, "****", records[0].Context["stringer"])
			assert.Equal(t, "****", records[0].Context["bytes"])
			assert.Equal(t, map[string]any{"value": "****"}, records[0].Context["escaped"])
			assert.Equal(t, map[string]any{"nested": []any{"****", map[string]any{"key": "Bearer ****"}}},
				records[0].Context[BodyAttr])
		})
	}
}

type maskTestStringer string

func (value maskTestStringer) String() string {
	return string(value)
}

func TestMaskEntryScope(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			root, buf := newMaskTestLogger(format)
			log := root.With(Mask("first"), zap.String("inherited", "first second third"))
			entry := log.Check(zapcore.InfoLevel, "first second third")
			require.NotNil(t, entry)

			entry.Write(zap.String("field", "first second third"), Mask("second"), Mask("third"))
			log.Info("first second third")

			records := maskTestRecords(t, format, buf.String())
			require.Len(t, records, 2)
			assert.Equal(t, "**** **** ****", records[0].Message)
			assert.Equal(t, "**** **** ****", records[0].Context["inherited"])
			assert.Equal(t, "**** **** ****", records[0].Context["field"])
			assert.NotContains(t, records[0].Context, "")
			assert.Equal(t, "**** second third", records[1].Message)
			assert.Equal(t, "**** second third", records[1].Context["inherited"])
		})
	}
}

type maskTestObject struct {
	value *string
	calls *int
}

func (obj maskTestObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	*obj.calls++
	enc.AddString("snapshot", *obj.value)
	enc.AddString("token", "secret")

	return nil
}

func TestMaskPreservesEvaluation(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		for _, lazy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/lazy=%t", format, lazy), func(t *testing.T) {
				root, buf := newMaskTestLogger(format)
				value, calls := "before", 0
				with := root.With

				if lazy {
					with = root.WithLazy
				}

				log := with(Mask("secret"), zap.Object("object", maskTestObject{value: &value, calls: &calls}))
				wantCalls, wantValue := 1, "before"

				if lazy {
					wantCalls, wantValue = 0, "after"
				}

				require.Equal(t, wantCalls, calls)

				value = "after"
				log.Info("secret")
				log.Info("secret")

				require.Equal(t, 1, calls)

				for _, record := range maskTestRecords(t, format, buf.String()) {
					assert.Equal(t, "****", record.Message)
					assert.Equal(t, map[string]any{"snapshot": wantValue, "token": "****"}, record.Context["object"])
				}
			})
		}
	}
}

func TestMaskCopiesArguments(t *testing.T) {
	root, buf := newMaskTestLogger("json")
	secrets := []string{"alpha", "beta"}
	field := Mask(secrets...)
	secrets[0] = "gamma"

	log := root.WithLazy(field)
	log.Info("alpha beta gamma")

	assert.Equal(t, "**** **** gamma", maskTestRecords(t, "json", buf.String())[0].Message)
}

func TestMaskHelpers(t *testing.T) {
	root, buf := newMaskTestLogger("json")

	root.With(Mask("secret")).
		ForHandler("secret-handler").ForHandler("secret-other-handler").
		ForConnection("secret-connection").ForConnection("secret-other-connection").
		ForAccount("secret-account").ForAccount("secret-other-account").Info("secret")

	record := maskTestRecords(t, "json", buf.String())[0]
	assert.Equal(t, "****", record.Message)
	assert.Equal(t, "****-other-handler", record.Handler)
	assert.Equal(t, "****-other-connection", record.Connection)
	assert.Equal(t, "****-other-account", record.Account)
}

func TestMaskConcurrentChildren(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			root, buf := newMaskTestLogger(format)
			parent := root.WithLazy(Mask("parent-secret"))
			var wg sync.WaitGroup

			for i := range 20 {
				wg.Go(func() {
					secret := fmt.Sprintf("child-secret-%03d", i)
					parent.With(Mask(secret)).Info("parent-secret "+secret, zap.String("token", secret))
				})
			}

			wg.Wait()

			records := maskTestRecords(t, format, buf.String())
			require.Len(t, records, 20)

			for _, record := range records {
				assert.Equal(t, "**** ****", record.Message)
				assert.Equal(t, "****", record.Context["token"])
			}

			buf.Reset()
			parent.Info("parent-secret child-secret-000")

			assert.Equal(t, "**** child-secret-000", maskTestRecords(t, format, buf.String())[0].Message)
		})
	}
}

func TestMaskJSONStrings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		secrets []string
		input   string
		want    string
	}{
		{
			name: "keys and numbers", secrets: []string{"message", "123", "secret"},
			input: `{"message":"message 123 secret","secret":123,"array":["secret",123,-123,1.23]}`,
			want:  `{"message":"**** **** ****","secret":123,"array":["****",123,-123,1.23]}`,
		},
		{
			name: "duplicate keys and whitespace", secrets: []string{"secret"},
			input: `{ "key": "secret", "key": "sec\u0072et" }`,
			want:  `{ "key": "****", "key": "****" }`,
		},
		{
			name: "longest match", secrets: []string{"abc", "abcdef", "abc", ""},
			input: `{"value":"abcdef abc abcdef"}`, want: `{"value":"**** **** ****"}`,
		},
		{
			name: "JSON punctuation", secrets: []string{"\\", "\"", "\n", "{"},
			input: `{"value":"a\\b\"c\nd{e","count":12}`, want: `{"value":"a****b****c****d****e","count":12}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			masked, err := maskJSONStrings([]byte(tc.input), maskRules{}.with(tc.secrets))
			require.NoError(t, err)
			defer masked.Free()

			assert.Equal(t, tc.want, masked.String())
			assert.True(t, json.Valid(masked.Bytes()))
		})
	}
}

func TestMaskEmpty(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			root, buf := newMaskTestLogger(format)
			root.Info("unchanged", zap.String("field", "value"))
			original := buf.String()

			buf.Reset()
			root.With(Mask(), Mask("", "")).Info("unchanged", zap.String("field", "value"), Mask(""))

			assert.Equal(t, original, buf.String())
		})
	}
}

func TestMaskConsoleFormatting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config func() zapcore.EncoderConfig
		msg    string
		fields bool
	}{
		{name: "metadata and stack", config: EncoderConfigConsole, msg: "secret {object}\nsecret", fields: true},
		{name: "without context", config: EncoderConfigConsole, msg: "secret"},
		{name: "empty message", config: EncoderConfigConsole, fields: true},
		{name: "context only", config: func() zapcore.EncoderConfig {
			return zapcore.EncoderConfig{LineEnding: "\r\n", ConsoleSeparator: " | "}
		}, fields: true},
		{name: "no line ending", config: func() zapcore.EncoderConfig {
			cfg := EncoderConfigConsole()
			cfg.SkipLineEnding = true
			cfg.ConsoleSeparator = " | "

			return cfg
		}, msg: "secret", fields: true},
		{name: "stack enabled", config: func() zapcore.EncoderConfig {
			cfg := EncoderConfigConsole()
			cfg.StacktraceKey, cfg.CallerKey, cfg.FunctionKey = "stack", "caller", "function"

			return cfg
		}, msg: "secret", fields: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.config()
			masked := newMaskingConsoleEncoder(cfg)
			plain := zapcore.NewConsoleEncoder(cfg)
			Mask("secret").AddTo(masked)

			entry := zapcore.Entry{
				Message: tc.msg, LoggerName: "secret.worker", Stack: "stack secret\nnext",
				Time: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), Level: zapcore.InfoLevel,
				Caller: zapcore.EntryCaller{Defined: true, File: "/some/secret.go", Line: 4, Function: "secret.fn"},
			}

			wantEntry := entry
			wantEntry.Message = strings.ReplaceAll(entry.Message, "secret", "****")
			wantEntry.LoggerName = "****.worker"
			wantEntry.Stack = "stack ****\nnext"
			wantEntry.Caller.File, wantEntry.Caller.Function = "/some/****.go", "****.fn"

			var fields, wantFields []zapcore.Field

			if tc.fields {
				zap.String("inherited", "secret").AddTo(masked)
				zap.String("inherited", "****").AddTo(plain)
				fields = []zapcore.Field{zap.Namespace("secret"), zap.String("token", "secret")}
				wantFields = []zapcore.Field{zap.Namespace("secret"), zap.String("token", "****")}
			}

			got, err := masked.EncodeEntry(entry, fields)
			require.NoError(t, err)
			defer got.Free()

			want, err := plain.EncodeEntry(wantEntry, wantFields)
			require.NoError(t, err)
			defer want.Free()

			assert.Equal(t, want.String(), got.String())
		})
	}
}

func FuzzMaskJSONStrings(f *testing.F) {
	f.Add("a secret in a message", "secret")
	f.Add("quotes \" and slash \\ and newline\n", "\n")
	f.Add("юникод ☺<>&", "☺")

	f.Fuzz(func(t *testing.T, value, secret string) {
		if secret == "" || !utf8.ValidString(value) || !utf8.ValidString(secret) {
			t.Skip()
		}

		data, err := json.Marshal([]any{value, value, 123})
		require.NoError(t, err)

		masked, err := maskJSONStrings(data, maskRules{}.with([]string{secret}))
		require.NoError(t, err)
		defer masked.Free()

		var decoded []any
		require.NoError(t, json.Unmarshal(masked.Bytes(), &decoded))

		want := strings.ReplaceAll(value, secret, maskReplacement)
		assert.Equal(t, []any{want, want, float64(123)}, decoded)
	})
}
