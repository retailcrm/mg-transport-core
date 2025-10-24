package beanstalk

import (
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/stretchr/testify/assert"
)

func TestQueue(t *testing.T) {
	manager := FakeManager{
		Addr: "address",
	}

	log := logger.NewDefault("json", true)
	queue := New(&manager, time.Hour, "test_queue", log)

	jobDone := make(chan bool, 1)
	type queueMsg struct {
		To   string `json:"to"`
		From string `json:"from"`
		Text string `json:"text"`
	}

	msg := queueMsg{
		To:   "to",
		From: "from",
		Text: "text",
	}

	go queue.Process(func(_ uint64, body []byte, done func()) {
		var actMsg queueMsg
		assert.NoError(t, json.Unmarshal(body, &actMsg))
		assert.Equal(t, msg, actMsg)
		done()
		jobDone <- true
	})

	_, err := queue.Put(msg)
	require.NoError(t, err)

	select {
	case <-jobDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for job to complete")
		return
	}
	assert.Equal(t, int64(1), manager.DeletedJobs.Load())

	assert.True(t, manager.TubeIsActive.Load())
	assert.True(t, manager.TubeSetIsActive.Load())
	queue.Shutdown()
	assert.False(t, manager.TubeIsActive.Load())
	assert.False(t, manager.TubeSetIsActive.Load())
}

func TestQueuePutError(t *testing.T) {
	manager := FakeManager{
		Addr: "address",

		PutJobErr: errors.New("put job error"),
	}

	log := logger.NewDefault("json", true)
	queue := New(&manager, time.Hour, "test_queue", log)

	type queueMsg struct {
		Text string `json:"text"`
	}
	msg := queueMsg{
		Text: "text",
	}
	_, err := queue.Put(msg)
	require.Error(t, err)
	assert.Equal(t, "put job error", err.Error())
}

func TestQueuePutNetError(t *testing.T) {
	manager := FakeManager{
		Addr: "address",

		PutJobErr: &net.DNSError{UnwrapErr: errors.New("put job error")},
	}

	log := logger.NewDefault("json", true)
	queue := New(&manager, time.Hour, "test_queue", log)

	type queueMsg struct {
		Text string `json:"text"`
	}
	msg := queueMsg{
		Text: "text",
	}

	go func() {
		_, err := queue.Put(msg)
		assert.Error(t, err)
		assert.Equal(t, "queue was stopped", err.Error())
	}()
	time.Sleep(time.Millisecond)

	queue.Shutdown()
	assert.NotEmpty(t, manager.ReconnectTubeTry.Load())
}

func TestQueueProcessError(t *testing.T) {
	manager := FakeManager{
		Addr: "address",

		GetJobErr: errors.New("get job error"),
	}

	log := logger.NewDefault("json", true)
	queue := New(&manager, time.Hour, "test_queue", log)

	type queueMsg struct {
		Text string `json:"text"`
	}
	msg := queueMsg{
		Text: "text",
	}

	go queue.Process(func(_ uint64, _ []byte, _ func()) {
		t.Error("should not happen")
	})

	_, err := queue.Put(msg)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)

	queue.Shutdown()
	assert.NotEmpty(t, manager.ReconnectTubeSetTry.Load())
}

func TestQueueFinishJobNetError(t *testing.T) {
	manager := FakeManager{
		Addr: "address",

		DeleteJobErr: &net.DNSError{UnwrapErr: errors.New("delete job error")},
	}

	log := logger.NewDefault("json", false)
	queue := New(&manager, time.Hour, "test_queue", log)

	type queueMsg struct {
		Text string `json:"text"`
	}
	msg := queueMsg{
		Text: "text",
	}

	_, err := queue.Put(msg)
	require.NoError(t, err)

	go func() {
		queue.finishJob(0)
	}()
	time.Sleep(time.Millisecond)

	queue.Shutdown()
	assert.NotEmpty(t, manager.ReconnectTubeSetTry.Load())
}
