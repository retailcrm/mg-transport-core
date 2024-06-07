package logger

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZap(format string, debug bool) *zap.Logger {
	switch format {
	case "json":
		return NewZapJSON(debug)
	case "console":
		return NewZapConsole(debug)
	default:
		panic(fmt.Sprintf("unknown logger format: %s", format))
	}
}

func NewZapConsole(debug bool) *zap.Logger {
	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}
	log, err := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      debug,
		Encoding:         "console",
		EncoderConfig:    EncoderConfigConsole(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
	if err != nil {
		panic(err)
	}
	return log
}

func EncoderConfigConsole() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey:    "message",
		LevelKey:      "level",
		TimeKey:       "datetime",
		NameKey:       "logger",
		CallerKey:     "caller",
		FunctionKey:   zapcore.OmitKey,
		StacktraceKey: "",
		LineEnding:    "\n",
		EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString("level_name=" + level.CapitalString())
		},
		EncodeTime: func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString("datetime=" + t.Format(time.RFC3339))
		},
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller: func(caller zapcore.EntryCaller, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString("caller=" + caller.TrimmedPath())
		},
		EncodeName:       zapcore.FullNameEncoder,
		ConsoleSeparator: " ",
	}
}

func NewZapJSON(debug bool) *zap.Logger {
	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}
	log, err := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      debug,
		Encoding:         "json-with-context",
		EncoderConfig:    EncoderConfigJSON(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
	if err != nil {
		panic(err)
	}
	return log
}

func EncoderConfigJSON() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey:       "message",
		LevelKey:         "level_name",
		TimeKey:          "datetime",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		StacktraceKey:    "",
		LineEnding:       "\n",
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeTime:       zapcore.RFC3339TimeEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
		EncodeName:       zapcore.FullNameEncoder,
		ConsoleSeparator: " ",
	}
}
