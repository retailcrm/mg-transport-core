package logger

import "log/slog"

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
