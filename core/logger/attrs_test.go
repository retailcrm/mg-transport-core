package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestStreamID(t *testing.T) {
	var cases = []struct {
		name   string
		input  interface{}
		result interface{}
	}{
		{
			name:   "empty",
			input:  "",
			result: "",
		},
		{
			name:   "string",
			input:  "test body",
			result: "test body",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val := StreamID(c.input)
			assert.Equal(t, StreamIDAttr, val.Key)
			assert.Equal(t, c.result, val.String)
		})
	}
}

func TestBody(t *testing.T) {
	var cases = []struct {
		name    string
		input   interface{}
		result  interface{}
		asserts func(t *testing.T, field zap.Field, input, result interface{})
	}{
		{
			name:   "empty string input",
			input:  "",
			result: nil,
		},
		{
			name:   "nil input",
			input:  nil,
			result: nil,
		},
		{
			name:   "string input",
			input:  "test body",
			result: "test body",
		},
		{
			name:   "json input",
			input:  `{"success":true}`,
			result: map[string]interface{}{"success": true},
		},
		{
			name:   "empty byte slice input",
			input:  []byte{},
			result: nil,
		},
		{
			name:   "byte slice input",
			input:  []byte("test body"),
			result: "test body",
		},
		{
			name:   "json.RawMessage input",
			input:  json.RawMessage("test body"),
			result: "test body",
		},
		{
			name:   "json byte slice input",
			input:  []byte(`{"success":true}`),
			result: map[string]interface{}{"success": true},
		},
		{
			name: "eof reader input",
			input: newReaderMock(func(p []byte) (n int, err error) {
				return 0, io.EOF
			}),
			result: nil,
		},
		{
			name:   "empty reader input",
			input:  newReaderMockData([]byte{}),
			result: nil,
		},
		{
			name:   "data reader input",
			input:  newReaderMockData([]byte("ooga booga")),
			result: "ooga booga",
		},
		{
			name:   "json data reader input",
			input:  newReaderMockData([]byte(`{"success":true}`)),
			result: map[string]interface{}{"success": true},
		},
		{
			name:   "check that seeker is rewound",
			input:  bytes.NewReader([]byte(`{"success":true}`)),
			result: map[string]interface{}{"success": true},
			asserts: func(t *testing.T, val zap.Field, input, result interface{}) {
				data, err := io.ReadAll(input.(io.Reader))
				require.NoError(t, err)
				assert.Equal(t, []byte(`{"success":true}`), data)
			},
		},
		{
			name:   "check that writer is rebuilt",
			input:  bytes.NewBuffer([]byte(`{"success":true}`)),
			result: map[string]interface{}{"success": true},
			asserts: func(t *testing.T, val zap.Field, input, result interface{}) {
				data, err := io.ReadAll(input.(io.Reader))
				require.NoError(t, err)
				assert.Equal(t, []byte(`{"success":true}`), data)
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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

			if c.asserts != nil {
				c.asserts(t, val, c.input, c.result)
			}
		})
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
