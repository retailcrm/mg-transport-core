package lifecycle

import (
	"context"
	"io"
	"net/http"
	"sync"
)

type shutdownRoundTripper struct {
	base http.RoundTripper
	ctx  context.Context
}

type shutdownResponseBody struct {
	io.ReadCloser
	cleanup func()
}

func (b *shutdownResponseBody) Read(data []byte) (int, error) {
	count, err := b.ReadCloser.Read(data)
	if err != nil {
		b.cleanup()
	}
	return count, err
}

func (b *shutdownResponseBody) Close() error {
	defer b.cleanup()
	return b.ReadCloser.Close()
}

func (t shutdownRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(request.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	cleanup := sync.OnceFunc(func() {
		stop()
		cancel()
	})

	response, err := t.base.RoundTrip(request.Clone(ctx))
	if err != nil || response == nil {
		cleanup()
		return response, err
	}
	if response.Body == nil {
		cleanup()
		return response, nil
	}

	response.Body = &shutdownResponseBody{ReadCloser: response.Body, cleanup: cleanup}
	return response, nil
}

// WrapHTTPClient returns a copy of client whose requests are canceled when the
// shutdown deadline expires or Stop is called. A nil client uses http.DefaultClient.
// The original client and its transport are not modified.
func (s *State) WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}

	cloned := *client
	base := cloned.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	cloned.Transport = shutdownRoundTripper{base: base, ctx: s.shutdownContext}
	return &cloned
}
