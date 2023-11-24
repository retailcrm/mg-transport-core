package logger

import (
	"fmt"

	v1 "github.com/retailcrm/mg-transport-api-client-go/v1"
)

type mgTransportClientAdapter struct {
	log Logger
}

func MGTransportClientAdapter(log Logger) v1.DebugLogger {
	return &mgTransportClientAdapter{log: log}
}

func (m *mgTransportClientAdapter) Debugf(msg string, args ...interface{}) {
	m.log.Debug(fmt.Sprintf(msg, args...))
}
