package health

const (
	// DefaultMinRequests is a default minimal threshold of total requests. If Counter has less than this amount of requests
	// total, it will be skipped because it can trigger false alerts otherwise.
	DefaultMinRequests = 10

	// DefaultFailureThreshold is a default value of successful requests that should be passed in order to suppress any
	// error notifications. If less than that percentage of requests are successful, the notification will be sent.
	DefaultFailureThreshold = 0.8
)

// CounterProcessor is a default implementation of Processor. It will try to localize the message in case of error.
type CounterProcessor struct {
	Localizer              NotifyMessageLocalizer
	Notifier               NotifyFunc
	ConnectionDataProvider ConnectionDataProvider
	Error                  string
	FailureThreshold       float64
	MinRequests            uint32
}

func (c CounterProcessor) Process(id int, counter Counter) {
	if counter.IsFailed() {
		if counter.IsFailureProcessed() {
			return
		}

		apiURL, apiKey, lang := c.ConnectionDataProvider(id)
		c.Notifier(apiURL, apiKey, c.getErrorText(counter.Message(), lang))
		counter.FailureProcessed()
		return
	}

	succeeded := counter.TotalSucceeded()
	failed := counter.TotalFailed()

	// Ignore this counter for now because total count of requests is less than minimal count.
	// The results may not be representative.
	if (succeeded + failed) < c.MinRequests {
		return
	}

	// If more than FailureThreshold % of requests are successful, don't do anything.
	// Default value is 0.8 which would be 80% of successful requests.
	if (float64(succeeded) / float64(succeeded+failed)) >= c.FailureThreshold {
		counter.ClearCountersProcessed()
		counter.FlushCounters()
		return
	}

	// Do not process counters values twice if error ocurred.
	if counter.IsCountersProcessed() {
		return
	}

	apiURL, apiKey, lang := c.ConnectionDataProvider(id)
	c.Notifier(apiURL, apiKey, c.getErrorText(c.Error, lang))
	counter.CountersProcessed()
	return
}

func (c CounterProcessor) getErrorText(msg, lang string) string {
	if c.Localizer == nil {
		return msg
	}
	c.Localizer.SetLocale(lang)
	return c.Localizer.GetLocalizedMessage(msg)
}
