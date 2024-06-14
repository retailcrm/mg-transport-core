package testutil

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type JSONRecordScannerTest struct {
	suite.Suite
}

func TestJSONRecordScanner(t *testing.T) {
	suite.Run(t, new(JSONRecordScannerTest))
}

func (t *JSONRecordScannerTest) new(lines []string) *JSONRecordScanner {
	return NewJSONRecordScanner(bytes.NewReader([]byte(strings.Join(lines, "\n"))))
}

func (t *JSONRecordScannerTest) newPredefined() *JSONRecordScanner {
	return t.new([]string{strings.ReplaceAll(`{
  "level_name": "ERROR",
  "datetime": "2024-06-07T13:49:17+03:00",
  "caller": "handlers/account_middleware.go:147",
  "message": "Cannot add account",
  "handler": "handlers.addAccount",
  "connection": "https://fake-uri.retailcrm.pro",
  "account": "@username",
  "context": {
    "body": "[]string{\"integration_read\", \"integration_write\"}",
    "statusCode": 500
  }
}`, "\n", "")})
}

func (t *JSONRecordScannerTest) assertPredefined(record LogRecord) {
	ts, err := time.Parse(time.RFC3339, "2024-06-07T13:49:17+03:00")
	t.Require().NoError(err)
	t.Assert().True(record.DateTime.Valid)
	t.Assert().Equal(ts, record.DateTime.Time)
	t.Assert().Equal("ERROR", record.LevelName)
	t.Assert().Equal("handlers/account_middleware.go:147", record.Caller)
	t.Assert().Equal("Cannot add account", record.Message)
	t.Assert().Equal("handlers.addAccount", record.Handler)
	t.Assert().Equal("https://fake-uri.retailcrm.pro", record.Connection)
	t.Assert().Equal("@username", record.Account)
	t.Assert().Equal("[]string{\"integration_read\", \"integration_write\"}", record.Context["body"])
	t.Assert().Equal(float64(500), record.Context["statusCode"])
}

func (t *JSONRecordScannerTest) TestScan_NotJSON() {
	rs := t.new([]string{"this is", "not json"})
	t.Assert().Error(rs.Scan())
}

func (t *JSONRecordScannerTest) TestScan_PartialJSON() {
	rs := t.new([]string{"{}", "not json"})
	t.Assert().NoError(rs.Scan())
	t.Assert().Error(rs.Scan())
}

func (t *JSONRecordScannerTest) TestScan_JSON() {
	rs := t.new([]string{"{}", "{}"})
	t.Assert().NoError(rs.Scan())
	t.Assert().NoError(rs.Scan())
	t.Assert().ErrorIs(rs.Scan(), io.EOF)
}

func (t *JSONRecordScannerTest) TestScan_JSONRecord() {
	rs := t.newPredefined()
	t.Assert().NoError(rs.Scan())
	t.Assert().ErrorIs(rs.Scan(), io.EOF)
	t.assertPredefined(rs.Entry())
}

func (t *JSONRecordScannerTest) TestScanAll_NotJSON() {
	rs := t.new([]string{"this is", "not json"})
	records, err := rs.ScanAll()
	t.Assert().Error(err)
	t.Assert().Empty(records)
}

func (t *JSONRecordScannerTest) TestScanAll_PartialJSON() {
	rs := t.new([]string{"{}", "not json"})
	records, err := rs.ScanAll()
	t.Assert().Error(err)
	t.Assert().Len(records, 1)
}

func (t *JSONRecordScannerTest) TestScanAll_JSON() {
	rs := t.new([]string{"{}", "{}"})
	records, err := rs.ScanAll()
	t.Assert().NoError(err)
	t.Assert().Len(records, 2)
}

func (t *JSONRecordScannerTest) TestScanAll_JSONRecord() {
	rs := t.newPredefined()
	records, err := rs.ScanAll()
	t.Assert().NoError(err)
	t.Assert().Len(records, 1)
	t.assertPredefined(records[0])
}
