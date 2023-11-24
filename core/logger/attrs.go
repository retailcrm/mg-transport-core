package logger

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"
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

func Err(err any) zap.Field {
	if err == nil {
		return zap.String(ErrorAttr, "<nil>")
	}
	return zap.Any(ErrorAttr, err)
}

func Handler(name string) zap.Field {
	return zap.String(HandlerAttr, name)
}

func HTTPStatusCode(code int) zap.Field {
	return zap.Int(HTTPStatusAttr, code)
}

func HTTPStatusName(code int) zap.Field {
	return zap.String(HTTPStatusNameAttr, http.StatusText(code))
}

func Body(val any) zap.Field {
	switch item := val.(type) {
	case string:
		return zap.String(BodyAttr, item)
	case []byte:
		return zap.String(BodyAttr, string(item))
	default:
		return zap.String(BodyAttr, fmt.Sprintf("%#v", val))
	}
}
