package logger

import (
	"fmt"

	metrics "github.com/retailcrm/zabbix-metrics-collector"
)

type zabbixCollectorAdapter struct {
	log Logger
}

func (a *zabbixCollectorAdapter) Errorf(format string, args ...interface{}) {
	baseMsg := "cannot send metrics to Zabbix"
	switch format {
	case "cannot send metrics to Zabbix: %v":
		baseMsg = "cannot stop collector"
		fallthrough
	case "cannot stop collector: %s":
		var err interface{}
		if len(args) > 0 {
			err = args[0]
		}
		a.log.Error(baseMsg, Err(err))
	default:
		a.log.Error(fmt.Sprintf(format, args...))
	}
}

// ZabbixCollectorAdapter works as a logger adapter for Zabbix metrics collector.
// It can extract error messages from Zabbix collector and convert them to structured format.
func ZabbixCollectorAdapter(log Logger) metrics.ErrorLogger {
	return &zabbixCollectorAdapter{log: log}
}
