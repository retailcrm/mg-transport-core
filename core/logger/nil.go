package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Nil struct{}

func NewNil() Logger {
	return &Nil{}
}

func (l *Nil) With(fields ...zap.Field) Logger {
	return l
}

func (l *Nil) WithLazy(fields ...zap.Field) Logger {
	return l
}

func (l *Nil) Level() zapcore.Level {
	return zapcore.DebugLevel
}

func (l *Nil) Check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry {
	return &zapcore.CheckedEntry{}
}

func (l *Nil) Log(lvl zapcore.Level, msg string, fields ...zap.Field) {}

func (l *Nil) Debug(msg string, fields ...zap.Field) {}

func (l *Nil) Info(msg string, fields ...zap.Field) {}

func (l *Nil) Warn(msg string, fields ...zap.Field) {}

func (l *Nil) Error(msg string, fields ...zap.Field) {}

func (l *Nil) DPanic(msg string, fields ...zap.Field) {}

func (l *Nil) Panic(msg string, fields ...zap.Field) {}

func (l *Nil) Fatal(msg string, fields ...zap.Field) {}

func (l *Nil) ForAccount(handler, conn, acc any) Logger {
	return l
}

func (l *Nil) Sync() error {
	return nil
}
