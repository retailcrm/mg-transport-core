package core

import (
	"github.com/retailcrm/mg-transport-core/core/errortools"
)

// ErrorResponse with the error message.
// Deprecated: use errortools.Response instead.
type ErrorResponse errortools.Response

// ErrorsResponse contains multiple errors in the list.
// Deprecated: use errortools.ListResponse instead.
type ErrorsResponse errortools.ListResponse

// GetErrorResponse returns ErrorResponse with specified status code
// Usage (with gin):
//      context.JSON(GetErrorResponse(http.StatusPaymentRequired, "Not enough money"))
// Deprecated: use errortools.GetErrorResponse instead.
func GetErrorResponse(statusCode int, err string) (int, interface{}) {
	return errortools.GetErrorResponse(statusCode, err)
}

// BadRequest returns ErrorResponse with code 400
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
// Deprecated: use errortools.BadRequest instead.
func BadRequest(err string) (int, interface{}) {
	return errortools.BadRequest(err)
}

// InternalServerError returns ErrorResponse with code 500
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
// Deprecated: use errortools.InternalServerError instead.
func InternalServerError(err string) (int, interface{}) {
	return errortools.InternalServerError(err)
}
