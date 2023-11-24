package logger

import (
	"io"

	"go.uber.org/zap/zapcore"
)

type writerAdapter struct {
	log   Logger
	level zapcore.Level
}

func WriterAdapter(log Logger, level zapcore.Level) io.Writer {
	return &writerAdapter{log: log, level: level}
}

func (w *writerAdapter) Write(p []byte) (n int, err error) {
	w.log.Log(w.level, string(p))
	return len(p), nil
}
