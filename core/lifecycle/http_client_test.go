package lifecycle

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestWrapHTTPClientCancelsAtShutdownDeadline(t *testing.T) {
	state := New()
	started := make(chan struct{})
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	client := &http.Client{Transport: base, Timeout: time.Second}
	wrapped := state.WrapHTTPClient(client)
	require.IsType(t, base, client.Transport)
	require.Equal(t, client.Timeout, wrapped.Timeout)
	require.NotSame(t, client, wrapped)

	done := make(chan error, 1)
	go func() {
		_, err := wrapped.Get("http://example.test")
		done <- err
	}()
	<-started
	ctx, cancel := context.WithCancel(t.Context())
	state.BeginShutdown(ctx)
	select {
	case <-done:
		t.Fatal("request canceled before shutdown context ended")
	default:
	}
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	state.Stop()
}

func TestWrapHTTPClientKeepsContextUntilBodyConsumed(t *testing.T) {
	state := New()
	requestContext := make(chan context.Context, 1)
	wrapped := state.WrapHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestContext <- request.Context()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}, nil
	})})

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test", nil)
	require.NoError(t, err)
	response, err := wrapped.Do(request)
	require.NoError(t, err)
	requestCtx := <-requestContext
	require.NoError(t, requestCtx.Err())
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
	require.ErrorIs(t, requestCtx.Err(), context.Canceled)
	require.NoError(t, response.Body.Close())
	state.Stop()
}

func TestWrapHTTPClientStopCancelsOutstandingRequest(t *testing.T) {
	state := New()
	started := make(chan struct{})
	wrapped := state.WrapHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})})
	done := make(chan error, 1)
	go func() {
		_, err := wrapped.Get("http://example.test")
		done <- err
	}()
	<-started
	state.Stop()
	require.ErrorIs(t, <-done, context.Canceled)
}
