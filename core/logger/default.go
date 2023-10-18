package logger

import (
	"context"
	"log/slog"
	"os"
)

type Default struct {
	Logger *slog.Logger
}

func NewDefault(log *slog.Logger) Logger {
	return &Default{Logger: log}
}

func NewDefaultText() Logger {
	return NewDefault(slog.New(slog.NewTextHandler(os.Stdout, DefaultOpts)))
}

func NewDefaultJSON() Logger {
	return NewDefault(slog.New(slog.NewJSONHandler(os.Stdout, DefaultOpts)))
}

func NewDefaultNil() Logger {
	return NewDefault(slog.New(NilHandler))
}

func (d *Default) Handler() slog.Handler {
	return d.Logger.Handler()
}

func (d *Default) ForAccount(handler, conn, acc any) Logger {
	return d.With(slog.Any(HandlerAttr, handler), slog.Any(ConnectionAttr, conn), slog.Any(AccountAttr, acc))
}

func (d *Default) With(args ...any) Logger {
	return &Default{Logger: d.Logger.With(args...)}
}

func (d *Default) WithGroup(name string) Logger {
	return &Default{Logger: d.Logger.WithGroup(name)}
}

func (d *Default) Enabled(ctx context.Context, level slog.Level) bool {
	return d.Logger.Enabled(ctx, level)
}

func (d *Default) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	d.Logger.Log(ctx, level, msg, args...)
}

func (d *Default) LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	d.Logger.LogAttrs(ctx, level, msg, attrs...)
}

func (d *Default) Debug(msg string, args ...any) {
	d.Logger.Debug(msg, args...)
}

func (d *Default) DebugContext(ctx context.Context, msg string, args ...any) {
	d.Logger.DebugContext(ctx, msg, args...)
}

func (d *Default) Info(msg string, args ...any) {
	d.Logger.Info(msg, args...)
}

func (d *Default) InfoContext(ctx context.Context, msg string, args ...any) {
	d.Logger.InfoContext(ctx, msg, args...)
}

func (d *Default) Warn(msg string, args ...any) {
	d.Logger.Warn(msg, args...)
}

func (d *Default) WarnContext(ctx context.Context, msg string, args ...any) {
	d.Logger.WarnContext(ctx, msg, args...)
}

func (d *Default) Error(msg string, args ...any) {
	d.Logger.Error(msg, args...)
}

func (d *Default) ErrorContext(ctx context.Context, msg string, args ...any) {
	d.Logger.ErrorContext(ctx, msg, args...)
}
