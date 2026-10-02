package beanstalk

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	beanstalk "github.com/beanstalkd/go-beanstalk"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/stretchr/testify/require"
)

func TestSettlementDoesNotWaitForReserve(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	reserveStarted := make(chan struct{})
	releaseReserve := make(chan struct{})
	var reservations atomic.Int32
	go func() {
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer connection.Close()
				scanner := bufio.NewScanner(connection)
				for scanner.Scan() {
					command := scanner.Text()
					switch {
					case strings.HasPrefix(command, "watch "):
						_, _ = connection.Write([]byte("WATCHING 2\r\n"))
					case strings.HasPrefix(command, "ignore "):
						_, _ = connection.Write([]byte("WATCHING 1\r\n"))
					case strings.HasPrefix(command, "reserve-with-timeout "):
						if reservations.Add(1) == 1 {
							_, _ = connection.Write([]byte("RESERVED 1 3\r\njob\r\n"))
						} else {
							close(reserveStarted)
							<-releaseReserve
							_, _ = connection.Write([]byte("TIMED_OUT\r\n"))
						}
					case strings.HasPrefix(command, "delete "):
						_, _ = connection.Write([]byte("DELETED\r\n"))
					}
				}
			}()
		}
	}()

	manager, err := NewManager(t.Context(), listener.Addr().String(), "jobs", logger.NewNil(), time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() {
		close(releaseReserve)
		_ = manager.Close()
	})
	id, _, err := manager.Reserve(time.Second)
	require.NoError(t, err)
	go func() { _, _, _ = manager.Reserve(5 * time.Second) }()
	select {
	case <-reserveStarted:
	case <-time.After(time.Second):
		t.Fatal("reserve did not start")
	}
	settled := make(chan error, 1)
	go func() { settled <- manager.Delete(id) }()
	select {
	case err := <-settled:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("settlement waited for reserve")
	}
}

func TestManagerAgainstBeanstalkd(t *testing.T) {
	address := os.Getenv("BEANSTALK_TEST_ADDR")
	if address == "" {
		t.Skip("set BEANSTALK_TEST_ADDR to test against beanstalkd")
	}
	manager, err := NewManager(t.Context(), address, fmt.Sprintf("test-%d", time.Now().UnixNano()), logger.NewNil(), time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, manager.Close()) })

	id, err := manager.Put([]byte("first"), 1024, 0, time.Second)
	require.NoError(t, err)
	reserved, body, err := manager.Reserve(time.Second)
	require.NoError(t, err)
	require.Equal(t, id, reserved)
	require.Equal(t, []byte("first"), body)
	require.NoError(t, manager.Touch(id))

	secondReserve := make(chan error, 1)
	go func() { _, _, reserveErr := manager.Reserve(time.Second); secondReserve <- reserveErr }()
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	require.NoError(t, manager.Delete(id))
	require.Less(t, time.Since(start), 500*time.Millisecond)
	require.ErrorIs(t, <-secondReserve, beanstalk.ErrTimeout)

	id, err = manager.Put([]byte("second"), 1024, 0, time.Second)
	require.NoError(t, err)
	reserved, _, err = manager.Reserve(time.Second)
	require.NoError(t, err)
	require.Equal(t, id, reserved)
	require.NoError(t, manager.Release(id, 1024, 0))
	reserved, _, err = manager.Reserve(time.Second)
	require.NoError(t, err)
	require.Equal(t, id, reserved)
	require.NoError(t, manager.Delete(id))
}
