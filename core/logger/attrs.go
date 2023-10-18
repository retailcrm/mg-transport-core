package logger

import (
	"fmt"
	"log/slog"
	"net/http"
)

const (
	HandlerAttr        = "handler"
	ConnectionAttr     = "connection"
	AccountAttr        = "account"
	CounterIDAttr      = "counterId"
	ErrorAttr          = "error"
	FailureMessageAttr = "failureMessage"
	BodyAttr           = "body"
	HTTPMethodAttr     = "method"
	HTTPStatusAttr     = "statusCode"
	HTTPStatusNameAttr = "statusName"
)

func Err(err any) slog.Attr {
	if err == nil {
		return slog.String(ErrorAttr, "<nil>")
	}
	return slog.Any(ErrorAttr, err)
}

func HTTPStatusCode(code int) slog.Attr {
	return slog.Int(HTTPStatusAttr, code)
}

func HTTPStatusName(code int) slog.Attr {
	return slog.String(HTTPStatusNameAttr, http.StatusText(code))
}

func Body(val any) slog.Attr {
	switch item := val.(type) {
	case string:
		return slog.String(BodyAttr, item)
	case []byte:
		return slog.String(BodyAttr, string(item))
	default:
		return slog.String(BodyAttr, fmt.Sprintf("%#v", val))
	}
}
