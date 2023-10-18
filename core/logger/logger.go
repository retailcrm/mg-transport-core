package logger

import (
	"context"
	"log/slog"
)

// LoggerOld contains methods which should be present in logger implementation.
type LoggerOld interface {
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Panic(args ...any)
	Panicf(format string, args ...any)
	Critical(args ...any)
	Criticalf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Warning(args ...any)
	Warningf(format string, args ...any)
	Notice(args ...any)
	Noticef(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Debug(args ...any)
	Debugf(format string, args ...any)
}

type Logger interface {
	Handler() slog.Handler
	With(args ...any) Logger
	WithGroup(name string) Logger
	ForAccount(handler, conn, acc any) Logger
	Enabled(ctx context.Context, level slog.Level) bool
	Log(ctx context.Context, level slog.Level, msg string, args ...any)
	LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
	Debug(msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
	Info(msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	Warn(msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	Error(msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}
