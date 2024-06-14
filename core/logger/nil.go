package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Nil logger doesn't do anything.
type Nil struct{}

// NewNil constructs new *Nil.
func NewNil() Logger {
	return &Nil{}
}

func (l *Nil) With(_ ...zap.Field) Logger {
	return l
}

func (l *Nil) WithLazy(_ ...zap.Field) Logger {
	return l
}

func (l *Nil) Level() zapcore.Level {
	return zapcore.DebugLevel
}

func (l *Nil) Check(_ zapcore.Level, _ string) *zapcore.CheckedEntry {
	return &zapcore.CheckedEntry{}
}

func (l *Nil) Log(_ zapcore.Level, _ string, _ ...zap.Field) {}

func (l *Nil) Debug(_ string, _ ...zap.Field) {}

func (l *Nil) Info(_ string, _ ...zap.Field) {}

func (l *Nil) Warn(_ string, _ ...zap.Field) {}

func (l *Nil) Error(_ string, _ ...zap.Field) {}

func (l *Nil) DPanic(_ string, _ ...zap.Field) {}

func (l *Nil) Panic(_ string, _ ...zap.Field) {}

func (l *Nil) Fatal(_ string, _ ...zap.Field) {}

func (l *Nil) ForHandler(_ any) Logger {
	return l
}

func (l *Nil) ForConnection(_ any) Logger {
	return l
}

func (l *Nil) ForAccount(_ any) Logger {
	return l
}

func (l *Nil) Sync() error {
	return nil
}
