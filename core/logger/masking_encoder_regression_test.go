package logger

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestMaskConsoleMetadataEvaluatedOnce(t *testing.T) {
	for _, masked := range []bool{false, true} {
		t.Run(fmt.Sprintf("masked=%t", masked), func(t *testing.T) {
			calls := map[string]int{}
			cfg := EncoderConfigConsole()
			cfg.CallerKey = "caller"
			cfg.EncodeTime = func(_ time.Time, enc zapcore.PrimitiveArrayEncoder) {
				calls["time"]++
				enc.AppendString(fmt.Sprintf("time-%d", calls["time"]))
			}
			cfg.EncodeLevel = func(_ zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
				calls["level"]++
				enc.AppendString(fmt.Sprintf("level-%d", calls["level"]))
			}
			cfg.EncodeName = func(_ string, enc zapcore.PrimitiveArrayEncoder) {
				calls["name"]++
				enc.AppendString(fmt.Sprintf("worker-%d", calls["name"]))
			}
			cfg.EncodeCaller = func(_ zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
				calls["caller"]++
				enc.AppendString(fmt.Sprintf("caller-%d", calls["caller"]))
			}
			enc := newMaskingConsoleEncoder(cfg)
			message, want := "secret", "secret"
			if masked {
				Mask("secret").AddTo(enc)
				want = "****"
			}
			entry := zapcore.Entry{Message: message, LoggerName: "gateway", Time: time.Now(),
				Caller: zapcore.EntryCaller{Defined: true, File: "gateway.go", Line: 42}}
			got, err := enc.EncodeEntry(entry, []zapcore.Field{zap.String("token", "secret")})
			require.NoError(t, err)
			defer got.Free()
			assert.Equal(t, map[string]int{"time": 1, "level": 1, "name": 1, "caller": 1}, calls)
			assert.Equal(t, fmt.Sprintf("time-1 level-1 worker-1 caller-1 %s {\"token\": \"%s\"}\n", want, want), got.String())
		})
	}
}

func TestMaskLineEnding(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		for _, ending := range []string{"\n", "\r\n", `"secret"`, `"unterminated`, ""} {
			t.Run(fmt.Sprintf("%s/%q", format, ending), func(t *testing.T) {
				cfg := EncoderConfigJSON()
				cfg.TimeKey, cfg.LevelKey = "", ""
				cfg.LineEnding, cfg.SkipLineEnding = ending, ending == ""
				var enc zapcore.Encoder
				if format == "json" {
					enc = NewJSONWithContextEncoder(cfg)
				} else {
					enc = newMaskingConsoleEncoder(cfg)
				}
				Mask("secret").AddTo(enc)
				got, err := enc.EncodeEntry(zapcore.Entry{Message: "secret"}, []zapcore.Field{zap.String("token", "secret")})
				require.NoError(t, err)
				defer got.Free()
				require.True(t, strings.HasSuffix(got.String(), ending), "got %q", got.String())
				body := strings.TrimSuffix(got.String(), ending)
				if format == "json" {
					assert.JSONEq(t, `{"message":"****","context":{"token":"****"}}`, body)
				} else {
					assert.Equal(t, "**** {\"token\": \"****\"}", body)
				}
			})
		}
	}
}

func TestMaskConsoleUnmaskedFormatting(t *testing.T) {
	for _, message := range []string{"", "request", `request {"token":"secret"}`, "request\nnext"} {
		for _, ending := range []string{"\n", "\r\n", ""} {
			t.Run(fmt.Sprintf("%q/%q", message, ending), func(t *testing.T) {
				cfg := EncoderConfigConsole()
				cfg.LineEnding, cfg.SkipLineEnding = ending, ending == ""
				cfg.StacktraceKey = "stack"
				actual, expected := newMaskingConsoleEncoder(cfg), zapcore.NewConsoleEncoder(cfg)
				fields := []zapcore.Field{zap.String("before", "secret"), zap.Namespace("nested"), zap.String("token", "secret")}
				for _, field := range fields {
					field.AddTo(actual)
					field.AddTo(expected)
				}
				entry := zapcore.Entry{Message: message, LoggerName: "gateway", Stack: "trace\nnext", Time: time.Now()}
				got, err := actual.EncodeEntry(entry, fields)
				require.NoError(t, err)
				defer got.Free()
				want, err := expected.EncodeEntry(entry, fields)
				require.NoError(t, err)
				defer want.Free()
				assert.Equal(t, want.String(), got.String())
			})
		}
	}
}
