package logger

import "log/slog"

var DefaultOpts = &slog.HandlerOptions{
	AddSource: false,
	Level:     slog.LevelDebug,
}
