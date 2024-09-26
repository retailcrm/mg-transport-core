package logger

import (
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a logging interface.
type Logger interface {
	// With adds fields to the logger and returns a new logger with those fields.
	With(fields ...zap.Field) Logger
	// WithLazy adds fields to the logger lazily and returns a new logger with those fields.
	WithLazy(fields ...zap.Field) Logger
	// Level returns the logging level of the logger.
	Level() zapcore.Level
	// Check checks if the log message meets the given level.
	Check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry
	// Log logs a message with the given level and fields.
	Log(lvl zapcore.Level, msg string, fields ...zap.Field)
	// Debug logs a debug-level message with the given fields.
	Debug(msg string, fields ...zap.Field)
	// Info logs an info-level message with the given fields.
	Info(msg string, fields ...zap.Field)
	// Warn logs a warning-level message with the given fields.
	Warn(msg string, fields ...zap.Field)
	// Error logs an error-level message with the given fields.
	Error(msg string, fields ...zap.Field)
	// DPanic logs a debug-panic-level message with the given fields and panics
	// if the logger's panic level is set to a non-zero value.
	DPanic(msg string, fields ...zap.Field)
	// Panic logs a panic-level message with the given fields and panics immediately.
	Panic(msg string, fields ...zap.Field)
	// Fatal logs a fatal-level message with the given fields, then calls os.Exit(1).
	Fatal(msg string, fields ...zap.Field)
	// ForHandler returns a new logger that is associated with the given handler.
	ForHandler(handler any) Logger
	// ForConnection returns a new logger that is associated with the given connection.
	ForConnection(conn any) Logger
	// ForAccount returns a new logger that is associated with the given account.
	ForAccount(acc any) Logger
	// Sync returns an error if there's a problem writing log messages to disk, or nil if all writes were successful.
	Sync() error
}

type previousField uint8

const (
	previousFieldHandler previousField = iota + 1
	previousFieldConnection
	previousFieldAccount
)

// Default is a default logger implementation.
type Default struct {
	*zap.Logger
	parent   *zap.Logger
	previous previousField
}

// NewDefault creates a new default logger with the given format and debug level.
func NewDefault(format string, debug bool) Logger {
	return &Default{
		Logger: NewZap(format, debug),
	}
}

// With adds fields to the logger and returns a new logger with those fields.
func (l *Default) With(fields ...zap.Field) Logger {
	return l.clone(l.Logger.With(fields...))
}

// WithLazy adds fields to the logger lazily and returns a new logger with those fields.
func (l *Default) WithLazy(fields ...zap.Field) Logger {
	return l.clone(l.Logger.WithLazy(fields...))
}

// ForHandler returns a new logger that is associated with the given handler.
// This will replace "handler" field if it was set before.
// Note: chain calls like ForHandler().With().ForHandler() will DUPLICATE handler field!
func (l *Default) ForHandler(handler any) Logger {
	if l.previous != previousFieldHandler {
		result := l.With(zap.Any(HandlerAttr, handler))
		result.(*Default).setPrevious(previousFieldHandler)
		result.(*Default).parent = l.Logger
		return result
	}
	result := l.clone(l.parentOrCurrent().With(zap.Any(HandlerAttr, handler)))
	result.(*Default).setPrevious(previousFieldHandler)
	return result
}

// ForConnection returns a new logger that is associated with the given connection.
// This will replace "connection" field if it was set before.
// Note: chain calls like ForConnection().With().ForConnection() will DUPLICATE connection field!
func (l *Default) ForConnection(conn any) Logger {
	if l.previous != previousFieldConnection {
		result := l.With(zap.Any(ConnectionAttr, conn))
		result.(*Default).setPrevious(previousFieldConnection)
		result.(*Default).parent = l.Logger
		return result
	}
	result := l.clone(l.parentOrCurrent().With(zap.Any(ConnectionAttr, conn)))
	result.(*Default).setPrevious(previousFieldConnection)
	return result
}

// ForAccount returns a new logger that is associated with the given account.
// This will replace "account" field if it was set before.
// Note: chain calls like ForAccount().With().ForAccount() will DUPLICATE account field!
func (l *Default) ForAccount(acc any) Logger {
	if l.previous != previousFieldAccount {
		result := l.With(zap.Any(AccountAttr, acc))
		result.(*Default).setPrevious(previousFieldAccount)
		result.(*Default).parent = l.Logger
		return result
	}
	result := l.clone(l.parentOrCurrent().With(zap.Any(AccountAttr, acc)))
	result.(*Default).setPrevious(previousFieldAccount)
	return result
}

// clone creates a copy of the given logger.
func (l *Default) clone(log *zap.Logger) Logger {
	parent := l.parent
	if parent == nil {
		parent = l.Logger
	}
	return &Default{Logger: log, parent: parent}
}

// parentOrCurrent returns parent logger if it exists or current logger otherwise.
func (l *Default) parentOrCurrent() *zap.Logger {
	if l.parent != nil {
		return l.parent
	}
	return l.Logger
}

func (l *Default) setPrevious(prev previousField) {
	l.previous = prev
}

// AnyZapFields converts an array of values to zap fields.
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
