package testutil

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
)

type testingTMock struct {
	logs   *bytes.Buffer
	failed bool
}

func (t *testingTMock) Log(args ...interface{}) {
	t.logs.WriteString(fmt.Sprintln(append([]interface{}{"=>"}, args...)...))
}

func (t *testingTMock) Logf(format string, args ...interface{}) {
	t.logs.WriteString(fmt.Sprintf(" => "+format, args...))
}

func (t *testingTMock) FailNow() {
	t.failed = true
}

func (t *testingTMock) Reset() {
	t.logs.Reset()
	t.failed = false
}

func (t *testingTMock) Logs() string {
	return t.logs.String()
}

func (t *testingTMock) Failed() bool {
	return t.failed
}

type AssertNoUnmatchedRequestsTest struct {
	suite.Suite
	tmock *testingTMock
}

func TestAssertNoUnmatchedRequests(t *testing.T) {
	suite.Run(t, new(AssertNoUnmatchedRequestsTest))
}

func (t *AssertNoUnmatchedRequestsTest) SetupSuite() {
	t.tmock = &testingTMock{logs: &bytes.Buffer{}}
}

func (t *AssertNoUnmatchedRequestsTest) SetupTest() {
	t.tmock.Reset()
	gock.CleanUnmatchedRequest()
}

func (t *AssertNoUnmatchedRequestsTest) Test_OK() {
	AssertNoUnmatchedRequests(t.tmock)

	t.Assert().Empty(t.tmock.Logs())
	t.Assert().False(t.tmock.Failed())
}

func (t *AssertNoUnmatchedRequestsTest) Test_HasUnmatchedRequests() {
	defer gock.Off()

	gock.New("https://example.com").Get("/nil").Reply(http.StatusOK)
	gock.New("https://example.com").
		Post("/dial").
		MatchHeader("X-Client-Data", "something").
		BodyString("something in body").
		Reply(http.StatusOK)

	res, err := http.Get("https://example.com/nil")
	t.Require().NoError(err)
	t.Assert().Equal(http.StatusOK, res.StatusCode)

	AssertNoUnmatchedRequests(t.tmock)

	t.Assert().True(gock.HasUnmatchedRequest())
	t.Assert().Empty(t.tmock.Logs())
	t.T().Log(t.tmock.Logs())
	t.Assert().True(t.tmock.Failed())
}
