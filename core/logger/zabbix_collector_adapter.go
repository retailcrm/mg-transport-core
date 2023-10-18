package logger

import (
	"fmt"

	metrics "github.com/retailcrm/zabbix-metrics-collector"
)

type zabbixCollectorAdapter struct {
	log Logger
}

func (a *zabbixCollectorAdapter) Errorf(format string, args ...interface{}) {
	a.log.Error(fmt.Sprintf(format, args...))
}

func ZabbixCollectorAdapter(log Logger) metrics.ErrorLogger {
	return &zabbixCollectorAdapter{log: log}
}
