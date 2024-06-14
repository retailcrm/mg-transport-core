package healthcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/retailcrm/mg-transport-core/v2/core/util/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CounterProcessorTest struct {
	suite.Suite
	apiURL string
	apiKey string
	lang   string
}

func TestCounterProcessor(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CounterProcessorTest))
}

func (t *CounterProcessorTest) SetupSuite() {
	t.apiURL = "https://test.retailcrm.pro"
	t.apiKey = "key"
	t.lang = "en"
}

func (t *CounterProcessorTest) localizer() NotifyMessageLocalizer {
	loc := &localizerMock{}
	loc.On("SetLocale", mock.AnythingOfType("string")).Return()
	loc.On("GetLocalizedTemplateMessage",
		mock.AnythingOfType("string"), mock.Anything).Return(
		func(msg string, tpl map[string]interface{}) string {
			data, err := json.Marshal(tpl)
			if err != nil {
				panic(err)
			}
			return fmt.Sprintf("%s [%s]", msg, string(data))
		})
	return loc
}

func (t *CounterProcessorTest) new(
	nf NotifyFunc, pr ConnectionDataProvider, noLocalizer ...bool) (Processor, *testutil.JSONRecordScanner) {
	loc := t.localizer()
	if len(noLocalizer) > 0 && noLocalizer[0] {
		loc = nil
	}

	log := testutil.NewBufferedLogger()
	return CounterProcessor{
		Localizer:              loc,
		Logger:                 log,
		Notifier:               nf,
		ConnectionDataProvider: pr,
		Error:                  "default error",
		FailureThreshold:       DefaultFailureThreshold,
		MinRequests:            DefaultMinRequests,
		Debug:                  true,
	}, testutil.NewJSONRecordScanner(log)
}

func (t *CounterProcessorTest) notifier(err ...error) *notifierMock {
	if len(err) > 0 && err[0] != nil {
		return &notifierMock{err: err[0]}
	}
	return &notifierMock{}
}

func (t *CounterProcessorTest) provider(notFound ...bool) ConnectionDataProvider {
	if len(notFound) > 0 && notFound[0] {
		return func(id int) (apiURL, apiKey, lang string, exists bool) {
			return "", "", "", false
		}
	}
	return func(id int) (apiURL, apiKey, lang string, exists bool) {
		return t.apiURL, t.apiKey, t.lang, true
	}
}

func (t *CounterProcessorTest) counter() mockedCounter {
	return &counterMock{}
}

func (t *CounterProcessorTest) Test_FailureProcessed() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(true)
	c.On("IsFailureProcessed").Return(true)

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 1)
	t.Assert().Contains(logs[0].Message, "skipping counter because its failure is already processed")
	t.Assert().Equal(float64(1), logs[0].Context["counterId"])
}

func (t *CounterProcessorTest) Test_CounterFailed_CannotFindConnection() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider(true))
	c := t.counter()
	c.On("IsFailed").Return(true)
	c.On("IsFailureProcessed").Return(false)

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 1)
	t.Assert().Contains(logs[0].Message, "cannot find connection data for counter")
	t.Assert().Equal(float64(1), logs[0].Context["counterId"])
}

func (t *CounterProcessorTest) Test_CounterFailed_ErrWhileNotifying() {
	n := t.notifier(errors.New("http status code: 500"))
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(true)
	c.On("IsFailureProcessed").Return(false)
	c.On("Message").Return("error message")
	c.On("FailureProcessed").Return()

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 1)
	t.Assert().Contains(logs[0].Message, "cannot send notification for counter")
	t.Assert().Equal(float64(1), logs[0].Context["counterId"])
	t.Assert().Equal("http status code: 500", logs[0].Context["error"])
	t.Assert().Equal("error message", logs[0].Context["failureMessage"])
	t.Assert().Equal(t.apiURL, n.apiURL)
	t.Assert().Equal(t.apiKey, n.apiKey)
	t.Assert().Equal("error message", n.message)
}

func (t *CounterProcessorTest) Test_CounterFailed_SentNotification() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(true)
	c.On("IsFailureProcessed").Return(false)
	c.On("Message").Return("error message")
	c.On("FailureProcessed").Return()

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 0)
	t.Assert().Equal(t.apiURL, n.apiURL)
	t.Assert().Equal(t.apiKey, n.apiKey)
	t.Assert().Equal("error message", n.message)
}

func (t *CounterProcessorTest) Test_TooFewRequests() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(0))
	c.On("TotalSucceeded").Return(uint32(DefaultMinRequests - 1))

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 1)
	t.Assert().Contains(logs[0].Message, "skipping counter because it has too few requests")
	t.Assert().Equal(float64(1), logs[0].Context["counterId"])
	t.Assert().Equal(float64(DefaultMinRequests), logs[0].Context["minRequests"])
}

func (t *CounterProcessorTest) Test_ThresholdNotPassed() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(20))
	c.On("TotalSucceeded").Return(uint32(80))
	c.On("ClearCountersProcessed").Return()
	c.On("FlushCounters").Return()

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 0)
	t.Assert().Empty(n.message)
}

func (t *CounterProcessorTest) Test_ThresholdPassed_AlreadyProcessed() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(21))
	c.On("TotalSucceeded").Return(uint32(79))
	c.On("IsCountersProcessed").Return(true)

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 0)
	t.Assert().Empty(n.message)
}

func (t *CounterProcessorTest) Test_ThresholdPassed_NoConnectionFound() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider(true))
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(21))
	c.On("TotalSucceeded").Return(uint32(79))
	c.On("IsCountersProcessed").Return(false)

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 1)
	t.Assert().Contains(logs[0].Message, "cannot find connection data for counter")
	t.Assert().Equal(float64(1), logs[0].Context["counterId"])
	t.Assert().Empty(n.message)
}

func (t *CounterProcessorTest) Test_ThresholdPassed_NotifyingError() {
	n := t.notifier(errors.New("unknown error"))
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(21))
	c.On("TotalSucceeded").Return(uint32(79))
	c.On("IsCountersProcessed").Return(false)
	c.On("Name").Return("MockedCounter")
	c.On("Message").Return("")
	c.On("CountersProcessed").Return()

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 1)
	t.Assert().Contains(logs[0].Message, "cannot send notification for counter")
	t.Assert().Equal(float64(1), logs[0].Context["counterId"])
	t.Assert().Equal("unknown error", logs[0].Context["error"])
	t.Assert().Equal(`default error [{"Name":"MockedCounter"}]`, n.message)
}

func (t *CounterProcessorTest) Test_ThresholdPassed_NotificationSent() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider())
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(21))
	c.On("TotalSucceeded").Return(uint32(79))
	c.On("IsCountersProcessed").Return(false)
	c.On("Name").Return("MockedCounter")
	c.On("CountersProcessed").Return()

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 0)
	t.Assert().Equal(`default error [{"Name":"MockedCounter"}]`, n.message)
}

func (t *CounterProcessorTest) Test_ThresholdPassed_NotificationSent_NoLocalizer() {
	n := t.notifier()
	p, log := t.new(n.Notify, t.provider(), true)
	c := t.counter()
	c.On("IsFailed").Return(false)
	c.On("TotalFailed").Return(uint32(21))
	c.On("TotalSucceeded").Return(uint32(79))
	c.On("IsCountersProcessed").Return(false)
	c.On("Name").Return("MockedCounter")
	c.On("CountersProcessed").Return()

	p.Process(1, c)
	c.AssertExpectations(t.T())

	logs, err := log.ScanAll()
	t.Require().NoError(err)
	t.Require().Len(logs, 0)
	t.Assert().Equal(`default error`, n.message)
}

type localizerMock struct {
	mock.Mock
}

func (l *localizerMock) SetLocale(lang string) {
	l.Called(lang)
}

func (l *localizerMock) GetLocalizedTemplateMessage(messageID string, templateData map[string]interface{}) string {
	args := l.Called(messageID, templateData)
	if fn, ok := args.Get(0).(func(string, map[string]interface{}) string); ok {
		return fn(messageID, templateData)
	}
	return args.String(0)
}

type mockedCounter interface {
	Counter
	On(methodName string, arguments ...interface{}) *mock.Call
	AssertExpectations(t mock.TestingT) bool
}

type counterMock struct {
	mock.Mock
}

func (cm *counterMock) Name() string {
	args := cm.Called()
	return args.String(0)
}

func (cm *counterMock) SetName(name string) {
	cm.Called(name)
}

func (cm *counterMock) HitSuccess() {
	cm.Called()
}

func (cm *counterMock) HitFailure() {
	cm.Called()
}

func (cm *counterMock) TotalSucceeded() uint32 {
	args := cm.Called()
	return args.Get(0).(uint32)
}

func (cm *counterMock) TotalFailed() uint32 {
	args := cm.Called()
	return args.Get(0).(uint32)
}

func (cm *counterMock) Message() string {
	args := cm.Called()
	return args.String(0)
}

func (cm *counterMock) IsFailed() bool {
	args := cm.Called()
	return args.Bool(0)
}

func (cm *counterMock) Failed(message string) {
	cm.Called(message)
}

func (cm *counterMock) IsFailureProcessed() bool {
	args := cm.Called()
	return args.Bool(0)
}

func (cm *counterMock) IsCountersProcessed() bool {
	args := cm.Called()
	return args.Bool(0)
}

func (cm *counterMock) FailureProcessed() {
	cm.Called()
}

func (cm *counterMock) CountersProcessed() {
	cm.Called()
}

func (cm *counterMock) ClearCountersProcessed() {
	cm.Called()
}

func (cm *counterMock) FlushCounters() {
	cm.Called()
}

type notifierMock struct {
	err     error
	apiURL  string
	apiKey  string
	message string
}

func (n *notifierMock) Notify(apiURL, apiKey, msg string) error {
	n.apiURL = apiURL
	n.apiKey = apiKey
	n.message = msg
	return n.err
}
