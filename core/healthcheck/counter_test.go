package healthcheck

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type AtomicCounterTest struct {
	suite.Suite
}

func TestAtomicCounter(t *testing.T) {
	suite.Run(t, new(AtomicCounterTest))
}

func (t *AtomicCounterTest) new() Counter {
	return NewAtomicCounter("test")
}

func (t *AtomicCounterTest) Test_Name() {
	t.Assert().Equal("test", t.new().Name())
}

func (t *AtomicCounterTest) Test_SetName() {
	c := t.new()
	c.SetName("new")
	t.Assert().Equal("new", c.Name())
}

func (t *AtomicCounterTest) Test_HitSuccess() {
	c := t.new()
	c.HitSuccess()
	t.Assert().Equal(uint32(1), c.TotalSucceeded())

	c.Failed("test")
	c.FailureProcessed()
	c.HitSuccess()
	t.Assert().Equal(uint32(2), c.TotalSucceeded())
	t.Assert().False(c.IsFailed())
	t.Assert().False(c.IsFailureProcessed())
	t.Assert().Equal("", c.Message())
}

func (t *AtomicCounterTest) Test_HitFailure() {
	c := t.new()
	c.HitFailure()
	t.Assert().Equal(uint32(1), c.TotalFailed())
	c.HitFailure()
	t.Assert().Equal(uint32(2), c.TotalFailed())
}

func (t *AtomicCounterTest) Test_Failed() {
	c := t.new()
	t.Require().False(c.IsFailed())
	t.Require().Equal("", c.Message())

	c.Failed("message")
	t.Assert().True(c.IsFailed())
	t.Assert().Equal("message", c.Message())
}

func (t *AtomicCounterTest) Test_CountersProcessed() {
	c := t.new()
	t.Require().False(c.IsCountersProcessed())

	c.CountersProcessed()
	t.Assert().True(c.IsCountersProcessed())

	c.ClearCountersProcessed()
	t.Assert().False(c.IsCountersProcessed())
}

func (t *AtomicCounterTest) Test_FlushCounters() {
	c := t.new()
	c.HitSuccess()
	t.Require().Equal(uint32(1), c.TotalSucceeded())

	c.FlushCounters()
	t.Assert().Equal(uint32(1), c.TotalSucceeded())

	c.(*AtomicCounter).timestamp.Store(time.Now().Add(-(DefaultResetPeriod + time.Second)))
	c.FlushCounters()
	t.Assert().Equal(uint32(0), c.TotalSucceeded())
}

func (t *AtomicCounterTest) Test_Concurrency() {
	c := t.new()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for i := 0; i < 1000; i++ {
			c.HitSuccess()
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 500; i++ {
			// this delay will ensure that failure is being called after success.
			// technically, both have been executed concurrently because first 399 calls will not be delayed.
			if i > 399 {
				time.Sleep(time.Microsecond)
			}
			if i > 400 {
				c.Failed("total failure")
				continue
			}
			c.HitFailure()
		}
		c.FailureProcessed()
		wg.Done()
	}()
	wg.Wait()

	t.Assert().Equal(uint32(1000), c.TotalSucceeded())
	t.Assert().Equal(uint32(401), c.TotalFailed())
	t.Assert().True(c.IsFailed())
	t.Assert().True(c.IsFailureProcessed())
	t.Assert().Equal("total failure", c.Message())
}
