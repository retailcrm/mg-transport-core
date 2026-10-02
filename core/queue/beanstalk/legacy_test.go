package beanstalk

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/require"
)

func TestLegacyBodyAdapter(t *testing.T) {
	manager := newFakeManager()
	legacyID, err := manager.Put([]byte(`{"text":"old"}`), 1, 0, time.Minute)
	require.NoError(t, err)
	driver := New(NewLegacyBodyAdapter(manager), queue.JSONCodec[struct {
		Text string `json:"text"`
	}]{}, Options{PollTimeout: time.Millisecond})

	legacy, err := driver.Dequeue(t.Context())
	require.NoError(t, err)
	require.Equal(t, "old", legacy.Value().Text)
	require.Equal(t, "legacy-"+strconv.FormatUint(legacyID, 10), legacy.Metadata().ID)
	require.NoError(t, legacy.Ack(t.Context()))

	require.NoError(t, driver.Enqueue(t.Context(), struct {
		Text string `json:"text"`
	}{Text: "new"}, queue.EnqueueOptions{ID: "new-id"}))
	modern, err := driver.Dequeue(t.Context())
	require.NoError(t, err)
	require.Equal(t, "new", modern.Value().Text)
	require.Equal(t, "new-id", modern.Metadata().ID)
	require.NoError(t, modern.Ack(t.Context()))
}

func TestLegacyBodyAdapterPreservesBinaryPayload(t *testing.T) {
	manager := newFakeManager()
	body := []byte{0x00, 0xff, 0x42}
	_, err := manager.Put(body, 1, 0, time.Minute)
	require.NoError(t, err)
	driver := New(NewLegacyBodyAdapter(manager), queue.BytesCodec{}, Options{PollTimeout: time.Millisecond})
	envelope, err := driver.Dequeue(t.Context())
	require.NoError(t, err)
	require.Equal(t, body, envelope.Value())
	require.NoError(t, envelope.Ack(t.Context()))
}

func TestLegacyBodyAdapterAgainstBeanstalkd(t *testing.T) {
	address := os.Getenv("BEANSTALK_TEST_ADDR")
	if address == "" {
		t.Skip("set BEANSTALK_TEST_ADDR to test against beanstalkd")
	}
	manager, err := NewManager(t.Context(), address,
		fmt.Sprintf("legacy-test-%d", time.Now().UnixNano()), logger.NewNil(), time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() { _ = manager.Close() })
	_, err = manager.Put([]byte(`"old job"`), 1, 0, time.Minute)
	require.NoError(t, err)
	driver := New(NewLegacyBodyAdapter(manager), queue.JSONCodec[string]{}, Options{})
	envelope, err := driver.Dequeue(t.Context())
	require.NoError(t, err)
	require.Equal(t, "old job", envelope.Value())
	require.NoError(t, envelope.Ack(t.Context()))
}
