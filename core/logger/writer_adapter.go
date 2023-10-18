package logger

import (
	"context"
	"io"
	"log/slog"
)

type writerAdapter struct {
	log   Logger
	level slog.Level
}

func WriterAdapter(log Logger, level slog.Level) io.Writer {
	return &writerAdapter{log: log, level: level}
}

func (w *writerAdapter) Write(p []byte) (n int, err error) {
	w.log.Log(context.Background(), w.level, string(p))
	return len(p), nil
}
