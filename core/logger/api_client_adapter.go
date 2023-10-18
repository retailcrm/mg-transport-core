package logger

import (
	"fmt"

	retailcrm "github.com/retailcrm/api-client-go/v2"
)

type apiClientAdapter struct {
	logger Logger
}

// APIClientAdapter returns BasicLogger that calls underlying logger.
func APIClientAdapter(logger Logger) retailcrm.BasicLogger {
	return &apiClientAdapter{logger: logger}
}

// Printf data in the log using Debug method.
func (l *apiClientAdapter) Printf(format string, v ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, v...))
}
