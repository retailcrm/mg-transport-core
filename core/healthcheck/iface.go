package healthcheck

var (
	// compile-time checks to ensure that implementations are compatible with the interface
	_ = Storage(&SyncMapStorage{})
	_ = Counter(&AtomicCounter{})
	_ = Processor(CounterProcessor{})
	_ = NotifyFunc(DefaultNotifyFunc)
	_ = CounterConstructor(NewAtomicCounter)
)

// Storage stores different instances of Counter. Implementation should be goroutine-safe.
type Storage interface {
	// Get counter by its ID. The counter will be instantiated automatically if necessary.
	// Name here is not used to identify the counter in the storage.
	Get(id int, name string) Counter
	// Remove counter if it exists.
	Remove(id int)
	// Process will iterate over counters and call Processor on each of them.
	// This method is used to collect counters data & send notifications.
	Process(processor Processor)
}

// Counter will count successful and failed requests. Its contents can be used to judge if specific entity (e.g. Connection / Account)
// is not working properly (invalid credentials, too many failed requests, etc) and take further action based on the result.
// Implementation should be goroutine-safe.
type Counter interface {
	// Name can be used as a more friendly identifier for the counter.
	Name() string
	// SetName of the counter.
	SetName(name string)
	// HitSuccess registers successful request. It should automatically clear error state because that state should be
	// used only if error is totally unrecoverable.
	HitSuccess()
	// HitFailure registers failed request.
	HitFailure()
	// TotalSucceeded returns how many requests were successful.
	TotalSucceeded() uint32
	// TotalFailed returns how many requests have failed.
	TotalFailed() uint32
	// Failed will put Counter into failed state with specific error message.
	Failed(message string)
	// IsFailed returns true if Counter is in failed state.
	IsFailed() bool
	// Message will return error message if Counter is in failed state.
	Message() string
	// IsFailureProcessed will return true if current error inside counter has been processed already.
	IsFailureProcessed() bool
	// FailureProcessed will mark current error inside Counter as processed.
	FailureProcessed()
	// IsCountersProcessed returns true if counters value has been processed by the checker.
	// This can be used if you want to process counter values only once.
	IsCountersProcessed() bool
	// CountersProcessed will mark current counters value as processed.
	CountersProcessed()
	// ClearCountersProcessed will set IsCountersProcessed to false.
	ClearCountersProcessed()
	// FlushCounters will reset request counters if deemed necessary (for example, AtomicCounter will clear counters
	// only if their contents are older than provided time period).
	// This won't clear IsCountersProcessed flag!
	FlushCounters()
}

// Processor is used to check if Counter is in error state and act accordingly.
type Processor interface {
	// Process counter data. This method is not goroutine-safe!
	Process(id int, counter Counter) bool
}

// NotifyMessageLocalizer is the smallest subset of core.Localizer used in the
type NotifyMessageLocalizer interface {
	SetLocale(locale string)
	GetLocalizedTemplateMessage(messageID string, templateData map[string]interface{}) string
}

// NotifyFunc will send notification about error to the system with provided credentials.
// It will send the notification to system admins.
type NotifyFunc func(apiURL, apiKey, msg string) error

// CounterConstructor is used to create counters. This way you can implement your own counter and still use default CounterStorage.
type CounterConstructor func(name string) Counter

// ConnectionDataProvider should return the connection credentials and language by counter ID.
// It's best to use account ID as a counter ID to be able to retrieve the necessary data as easy as possible.
type ConnectionDataProvider func(id int) (apiURL, apiKey, lang string, exists bool)
