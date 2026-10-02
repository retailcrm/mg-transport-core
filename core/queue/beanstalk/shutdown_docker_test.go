package beanstalk

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/require"
)

// Run with an isolated disposable container:
// Set BEANSTALK_TEST_ADDR and BEANSTALK_TEST_CONTAINER for a disposable Docker container,
// then run go test ./core/queue/beanstalk -run TestCloseIntakeDuringBeanstalkOutage -count=1.
func TestCloseIntakeDuringBeanstalkOutage(t *testing.T) {
	address, container := os.Getenv("BEANSTALK_TEST_ADDR"), os.Getenv("BEANSTALK_TEST_CONTAINER")
	if address == "" || container == "" {
		t.Skip("set BEANSTALK_TEST_ADDR and BEANSTALK_TEST_CONTAINER to test with Docker")
	}
	manager, err := NewManager(t.Context(), address, "shutdown-test", logger.NewNil(), 20*time.Millisecond)
	require.NoError(t, err)
	q := queue.New(1, New(manager, queue.JSONCodec[string]{}, Options{}))

	command := exec.CommandContext(t.Context(), "docker", "stop", "-t", "0", container)
	require.NoError(t, command.Run())
	t.Cleanup(func() { _ = exec.Command("docker", "start", container).Run() })

	ctx, cancel := context.WithCancel(t.Context())
	enqueued := make(chan error, 1)
	go func() { enqueued <- q.Enqueue(ctx, "job") }()
	time.Sleep(100 * time.Millisecond)
	waiting := make(chan error, 1)
	go func() { waiting <- q.Enqueue(ctx, "second job") }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	closed := make(chan struct{})
	go func() { q.CloseIntake(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		_ = manager.Close()
		<-closed
		t.Fatal("CloseIntake blocked after the enqueue context was canceled")
	}
	for _, result := range []<-chan error{enqueued, waiting} {
		select {
		case err := <-result:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			_ = manager.Close()
			t.Fatal("Enqueue did not stop after its context was canceled")
		}
	}
	_ = q.Close(t.Context())
}
