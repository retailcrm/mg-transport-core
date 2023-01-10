package healthcheck

import (
	"time"

	"go.uber.org/atomic"
)

// DefaultResetPeriod is a default period for AtomicCounter after which internal request counters will be reset.
const DefaultResetPeriod = time.Minute * 15

// AtomicCounter is a default Counter implementation.
// It uses atomics under the hood (hence the name) and can be configured with custom reset timeout and
type AtomicCounter struct {
	name              atomic.String
	msg               atomic.String
	timestamp         atomic.Time
	resetPeriod       time.Duration
	failure           atomic.Uint32
	failed            atomic.Bool
	failureProcessed  atomic.Bool
	countersProcessed atomic.Bool
	success           atomic.Uint32
}

// NewAtomicCounterWithPeriod returns AtomicCounter configured with provided period.
func NewAtomicCounterWithPeriod(name string, resetPeriod time.Duration) Counter {
	c := &AtomicCounter{}
	c.SetName(name)
	c.resetPeriod = resetPeriod
	c.timestamp.Store(time.Now())
	return c
}

// NewAtomicCounter returns AtomicCounter with DefaultResetPeriod.
func NewAtomicCounter(name string) Counter {
	return NewAtomicCounterWithPeriod(name, DefaultResetPeriod)
}

func (a *AtomicCounter) Name() string {
	return a.name.Load()
}

func (a *AtomicCounter) SetName(name string) {
	a.name.Store(name)
}

func (a *AtomicCounter) HitSuccess() {
	a.success.Add(1)
	if a.failed.CompareAndSwap(true, false) {
		a.failureProcessed.Store(false)
		a.msg.Store("")
	}
}

func (a *AtomicCounter) HitFailure() {
	a.failure.Add(1)
}

func (a *AtomicCounter) TotalSucceeded() uint32 {
	return a.success.Load()
}

func (a *AtomicCounter) TotalFailed() uint32 {
	return a.failure.Load()
}

func (a *AtomicCounter) Failed(message string) {
	a.msg.Store(message)
	a.failed.Store(true)
}

func (a *AtomicCounter) IsFailed() bool {
	return a.failed.Load()
}

func (a *AtomicCounter) Message() string {
	return a.msg.Load()
}

func (a *AtomicCounter) IsFailureProcessed() bool {
	return a.failureProcessed.Load()
}

func (a *AtomicCounter) FailureProcessed() {
	a.failureProcessed.Store(true)
}

func (a *AtomicCounter) IsCountersProcessed() bool {
	return a.countersProcessed.Load()
}

func (a *AtomicCounter) CountersProcessed() {
	a.countersProcessed.Store(true)
}

func (a *AtomicCounter) ClearCountersProcessed() {
	a.countersProcessed.Store(false)
}

func (a *AtomicCounter) FlushCounters() {
	if time.Now().After(a.timestamp.Load().Add(a.resetPeriod)) {
		a.timestamp.Store(time.Now())
		a.success.Store(0)
		a.failure.Store(0)
	}
}
