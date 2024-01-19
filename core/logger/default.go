package logger

import (
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	With(fields ...zap.Field) Logger
	WithLazy(fields ...zap.Field) Logger
	Level() zapcore.Level
	Check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry
	Log(lvl zapcore.Level, msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	DPanic(msg string, fields ...zap.Field)
	Panic(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	ForHandler(handler any) Logger
	ForConnection(conn any) Logger
	ForAccount(acc any) Logger
	Sync() error
}

type Default struct {
	*zap.Logger
}

func NewDefault(debug bool) Logger {
	return &Default{
		Logger: NewZap(debug),
	}
}

func (l *Default) With(fields ...zap.Field) Logger {
	return l.With(fields...).(Logger)
}

func (l *Default) WithLazy(fields ...zap.Field) Logger {
	return l.WithLazy(fields...).(Logger)
}

func (l *Default) ForHandler(handler any) Logger {
	return l.WithLazy(zap.Any(HandlerAttr, handler))
}

func (l *Default) ForConnection(conn any) Logger {
	return l.WithLazy(zap.Any(ConnectionAttr, conn))
}

func (l *Default) ForAccount(acc any) Logger {
	return l.WithLazy(zap.Any(AccountAttr, acc))
}

func AnyZapFields(args []interface{}) []zap.Field {
	fields := make([]zap.Field, len(args))
	for i := 0; i < len(fields); i++ {
		if val, ok := args[i].(zap.Field); ok {
			fields[i] = val
			continue
		}
		fields[i] = zap.Any("arg"+strconv.Itoa(i), args[i])
	}
	return fields
}
