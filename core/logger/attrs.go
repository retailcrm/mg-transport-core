package logger

import (
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
	HTTPMethodAttr     = "httpMethod"
	HTTPStatusAttr     = "httpStatusCode"
	HTTPStatusNameAttr = "httpStatusName"
)

func ErrAttr(err any) slog.Attr {
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
	return slog.Any(BodyAttr, val)
}
