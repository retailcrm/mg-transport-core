package logger

import (
	"fmt"
	"io"
	"net/http"

	json "github.com/goccy/go-json"

	"go.uber.org/zap"
)

// HandlerAttr represents the attribute name for the handler.
const HandlerAttr = "handler"

// ConnectionAttr represents the attribute name for the connection.
const ConnectionAttr = "connection"

// AccountAttr represents the attribute name for the account.
const AccountAttr = "account"

// CounterIDAttr represents the attribute name for the counter ID.
const CounterIDAttr = "counterId"

// ErrorAttr represents the attribute name for an error.
const ErrorAttr = "error"

// FailureMessageAttr represents the attribute name for a failure message.
const FailureMessageAttr = "failureMessage"

// BodyAttr represents the attribute name for the request body.
const BodyAttr = "body"

// HTTPMethodAttr represents the attribute name for the HTTP method.
const HTTPMethodAttr = "method"

// HTTPStatusAttr represents the attribute name for the HTTP status code.
const HTTPStatusAttr = "statusCode"

// HTTPStatusNameAttr represents the attribute name for the HTTP status name.
const HTTPStatusNameAttr = "statusName"

// Err returns a zap.Field with the given error value.
func Err(err any) zap.Field {
	if err == nil {
		return zap.String(ErrorAttr, "<nil>")
	}
	return zap.Any(ErrorAttr, err)
}

// Handler returns a zap.Field with the given handler name.
func Handler(name string) zap.Field {
	return zap.String(HandlerAttr, name)
}

// HTTPStatusCode returns a zap.Field with the given HTTP status code.
func HTTPStatusCode(code int) zap.Field {
	return zap.Int(HTTPStatusAttr, code)
}

// HTTPStatusName returns a zap.Field with the given HTTP status name.
func HTTPStatusName(code int) zap.Field {
	return zap.String(HTTPStatusNameAttr, http.StatusText(code))
}

// Body returns a zap.Field with the given request body value.
func Body(val any) zap.Field {
	switch item := val.(type) {
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(item), &m); err == nil {
			return zap.Any(BodyAttr, m)
		}
		return zap.String(BodyAttr, item)
	case []byte:
		var m interface{}
		if err := json.Unmarshal(item, &m); err == nil {
			return zap.Any(BodyAttr, m)
		}
		return zap.String(BodyAttr, string(item))
	case io.Reader:
		data, err := io.ReadAll(item)
		if err != nil {
			return zap.String(BodyAttr, fmt.Sprintf("%#v", val))
		}
		var m interface{}
		if err := json.Unmarshal(data, &m); err == nil {
			return zap.Any(BodyAttr, m)
		}
		return zap.String(BodyAttr, string(data))
	default:
		return zap.String(BodyAttr, fmt.Sprintf("%#v", val))
	}
}
