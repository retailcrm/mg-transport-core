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

func HTTPStatus(code int) []any {
	return []any{slog.Int(HTTPStatusAttr, code), slog.String(HTTPStatusNameAttr, http.StatusText(code))}
}
