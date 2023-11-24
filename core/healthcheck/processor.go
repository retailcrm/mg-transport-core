package healthcheck

import (
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"go.uber.org/zap"
)

const (
	// DefaultMinRequests is a default minimal threshold of total requests. If Counter has less than
	// this amount of requests total, it will be skipped because it can trigger false alerts otherwise.
	DefaultMinRequests = 10

	// DefaultFailureThreshold is a default value of successful requests that should be passed in order to suppress any
	// error notifications. If less than that percentage of requests are successful, the notification will be sent.
	DefaultFailureThreshold = 0.8
)

// CounterProcessor is a default implementation of Processor. It will try to localize the message in case of error.
type CounterProcessor struct {
	Localizer              NotifyMessageLocalizer
	Logger                 logger.Logger
	Notifier               NotifyFunc
	ConnectionDataProvider ConnectionDataProvider
	Error                  string
	FailureThreshold       float64
	MinRequests            uint32
	Debug                  bool
}

func (c CounterProcessor) Process(id int, counter Counter) bool { // nolint:varnamelen
	if counter.IsFailed() {
		if counter.IsFailureProcessed() {
			c.debugLog("skipping counter because its failure is already processed", zap.Int(logger.CounterIDAttr, id))
			return true
		}

		apiURL, apiKey, _, exists := c.ConnectionDataProvider(id)
		if !exists {
			c.debugLog("cannot find connection data for counter", zap.Int(logger.CounterIDAttr, id))
			return true
		}
		err := c.Notifier(apiURL, apiKey, counter.Message())
		if err != nil {
			c.debugLog("cannot send notification for counter",
				zap.Int(logger.CounterIDAttr, id), logger.Err(err), zap.String(logger.FailureMessageAttr, counter.Message()))
		}
		counter.FailureProcessed()
		return true
	}

	succeeded := counter.TotalSucceeded()
	failed := counter.TotalFailed()

	// Ignore this counter for now because total count of requests is less than minimal count.
	// The results may not be representative.
	if (succeeded + failed) < c.MinRequests {
		c.debugLog("skipping counter because it has too few requests",
			zap.Int(logger.CounterIDAttr, id), zap.Any("minRequests", c.MinRequests))
		return true
	}

	// If more than FailureThreshold % of requests are successful, don't do anything.
	// Default value is 0.8 which would be 80% of successful requests.
	if (float64(succeeded) / float64(succeeded+failed)) >= c.FailureThreshold {
		counter.ClearCountersProcessed()
		counter.FlushCounters()
		return true
	}

	// Do not process counters values twice if error occurred.
	if counter.IsCountersProcessed() {
		return true
	}

	apiURL, apiKey, lang, exists := c.ConnectionDataProvider(id)
	if !exists {
		c.debugLog("cannot find connection data for counter", zap.Int(logger.CounterIDAttr, id))
		return true
	}
	err := c.Notifier(apiURL, apiKey, c.getErrorText(counter.Name(), c.Error, lang))
	if err != nil {
		c.debugLog("cannot send notification for counter",
			zap.Int(logger.CounterIDAttr, id), logger.Err(err), zap.String(logger.FailureMessageAttr, counter.Message()))
	}
	counter.CountersProcessed()
	return true
}

func (c CounterProcessor) getErrorText(name, msg, lang string) string {
	if c.Localizer == nil {
		return msg
	}
	c.Localizer.SetLocale(lang)
	return c.Localizer.GetLocalizedTemplateMessage(msg, map[string]interface{}{
		"Name": name,
	})
}

func (c CounterProcessor) debugLog(msg string, args ...interface{}) {
	if c.Debug {
		c.Logger.Debug(msg, logger.AnyZapFields(args)...)
	}
}
