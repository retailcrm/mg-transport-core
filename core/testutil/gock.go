package testutil

import (
	"fmt"
	"io"

	"gopkg.in/h2non/gock.v1"
)

// UnmatchedRequestsTestingT contains all of *testing.T methods which are needed for AssertNoUnmatchedRequests.
type UnmatchedRequestsTestingT interface {
	Log(...interface{})
	Logf(string, ...interface{})
	FailNow()
}

// AssertNoUnmatchedRequests check that every request in the gock mock chain has been matched.
// It will print out an entire request data for every unmatched request.
func AssertNoUnmatchedRequests(t UnmatchedRequestsTestingT) {
	if gock.HasUnmatchedRequest() { // nolint:nestif
		t.Log("gock has unmatched requests. their contents will be dumped here.\n")

		for _, r := range gock.GetUnmatchedRequests() {
			t.Logf("%s %s %s\n", r.Proto, r.Method, r.URL.String())
			t.Logf(" > RemoteAddr: %s\n", r.RemoteAddr)
			t.Logf(" > Host: %s\n", r.Host)
			t.Logf(" > Length: %d\n", r.ContentLength)

			for _, encoding := range r.TransferEncoding {
				t.Logf(" > Transfer-Encoding: %s\n", encoding)
			}

			for header, values := range r.Header {
				for _, value := range values {
					t.Logf("[header] %s: %s\n", header, value)
				}
			}

			if r.Body == nil {
				t.Log("No body is present.")
			} else {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Logf("Cannot read body: %s\n", err)
				}

				if len(data) == 0 {
					t.Log("Body is empty.")
				} else {
					t.Logf("Body:\n%s\n", string(data))
				}
			}

			for header, values := range r.Trailer {
				for _, value := range values {
					t.Logf("[trailer header] %s: %s\n", header, value)
				}
			}

			fmt.Println()
		}

		t.FailNow()
	}
}
