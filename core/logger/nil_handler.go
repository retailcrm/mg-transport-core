package logger

import (
	"context"
	"log/slog"
)

var NilHandler slog.Handler = &nilHandler{}

type nilHandler struct{}

func (n *nilHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return false
}

func (n *nilHandler) Handle(ctx context.Context, record slog.Record) error {
	return nil
}

func (n *nilHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return n
}

func (n *nilHandler) WithGroup(name string) slog.Handler {
	return n
}
