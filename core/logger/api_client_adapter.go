package logger

import (
	"fmt"

	"go.uber.org/zap"

	retailcrm "github.com/retailcrm/api-client-go/v2"
)

const (
	apiDebugLogReq  = "API Request: %s %s"
	apiDebugLogResp = "API Response: %s"
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
	switch format {
	case apiDebugLogReq:
		var url, key string
		if len(v) > 0 {
			url = fmt.Sprint(v[0])
		}
		if len(v) > 1 {
			key = fmt.Sprint(v[1])
		}
		l.logger.Debug("API Request", zap.String("url", url), zap.String("key", key))
	case apiDebugLogResp:
		l.logger.Debug("API Response", Body(v[0]))
	default:
		l.logger.Debug(fmt.Sprintf(format, v...))
	}
}
