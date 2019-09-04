package core

import (
	"os"

	"github.com/op/go-logging"
)

// NewLogger will create new logger with specified formatter.
// Usage:
//	    logger := NewLogger(config, DefaultLogFormatter())
func NewLogger(transportCode string, logLevel logging.Level, logFormat logging.Formatter) *logging.Logger {
	logger := logging.MustGetLogger(transportCode)
	logBackend := logging.NewLogBackend(os.Stdout, "", 0)
	formatBackend := logging.NewBackendFormatter(logBackend, logFormat)
	backend1Leveled := logging.AddModuleLevel(logBackend)
	backend1Leveled.SetLevel(logLevel, "")
	logging.SetBackend(formatBackend)

	return logger
}

// DefaultLogFormatter will return default formatter for logs
func DefaultLogFormatter() logging.Formatter {
	return logging.MustStringFormatter(
		`%{time:2006-01-02 15:04:05.000} %{level:.4s} => %{message}`,
	)
}
