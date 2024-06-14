package logger

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"testing"
)

func TestErr(t *testing.T) {
	var cases = []struct {
		source   interface{}
		expected string
	}{
		{
			source:   nil,
			expected: "<nil>",
		},
		{
			source:   errors.New("untimely error"),
			expected: "untimely error",
		},
	}

	for _, c := range cases {
		val := Err(c.source)
		assert.Equal(t, c.expected, func() string {
			if val.String != "" {
				return val.String
			}
			if val.Interface != nil {
				return fmt.Sprintf("%s", val.Interface)
			}
			return ""
		}())
		assert.Equal(t, ErrorAttr, val.Key)
	}
}

func TestHandler(t *testing.T) {
	val := Handler("handlerName")
	assert.Equal(t, HandlerAttr, val.Key)
	assert.Equal(t, "handlerName", val.String)
}

func TestHTTPStatusCode(t *testing.T) {
	val := HTTPStatusCode(http.StatusOK)
	assert.Equal(t, HTTPStatusAttr, val.Key)
	assert.Equal(t, http.StatusOK, int(val.Integer))
}

func TestHTTPStatusName(t *testing.T) {
	val := HTTPStatusName(http.StatusOK)
	assert.Equal(t, HTTPStatusNameAttr, val.Key)
	assert.Equal(t, http.StatusText(http.StatusOK), val.String)
}

func TestBody(t *testing.T) {
	var cases = []struct {
		input  interface{}
		result interface{}
	}{
		{
			input:  "",
			result: nil,
		},
		{
			input:  nil,
			result: nil,
		},
		{
			input:  "ooga booga",
			result: "ooga booga",
		},
		{
			input:  `{"success":true}`,
			result: map[string]interface{}{"success": true},
		},
		{
			input:  []byte{},
			result: nil,
		},
		{
			input:  nil,
			result: nil,
		},
		{
			input:  []byte("ooga booga"),
			result: "ooga booga",
		},
		{
			input:  []byte(`{"success":true}`),
			result: map[string]interface{}{"success": true},
		},
		{
			input: newReaderMock(func(p []byte) (n int, err error) {
				return 0, io.EOF
			}),
			result: nil,
		},
		{
			input:  newReaderMockData([]byte{}),
			result: nil,
		},
		{
			input:  newReaderMockData([]byte("ooga booga")),
			result: "ooga booga",
		},

		{
			input:  newReaderMockData([]byte(`{"success":true}`)),
			result: map[string]interface{}{"success": true},
		},
	}
	for _, c := range cases {
		val := Body(c.input)
		assert.Equal(t, BodyAttr, val.Key)

		switch assertion := c.result.(type) {
		case string:
			assert.Equal(t, assertion, val.String)
		case int:
			assert.Equal(t, assertion, int(val.Integer))
		default:
			assert.Equal(t, c.result, val.Interface)
		}
	}
}

type readerMock struct {
	mock.Mock
}

func newReaderMock(cb func(p []byte) (n int, err error)) io.Reader {
	r := &readerMock{}
	r.On("Read", mock.Anything).Return(cb)
	return r
}

func newReaderMockData(data []byte) io.Reader {
	return newReaderMock(bytes.NewReader(data).Read)
}

func (m *readerMock) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	out := args.Get(0)
	if cb, ok := out.(func(p []byte) (n int, err error)); ok {
		return cb(p)
	}
	return args.Int(0), args.Error(1)
}
