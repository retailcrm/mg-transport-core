package errorutil

import "net/http"

// Response with the error message.
type Response struct {
	Error string `json:"error"`
}

// ListResponse contains multiple errors in the list.
type ListResponse struct {
	Error []string `json:"error"`
}

// GetErrorResponse returns ErrorResponse with specified status code
// Usage (with gin):
//      context.JSON(GetErrorResponse(http.StatusPaymentRequired, "Not enough money"))
func GetErrorResponse(statusCode int, err string) (int, interface{}) {
	return statusCode, Response{
		Error: err,
	}
}

// BadRequest returns ErrorResponse with code 400
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
func BadRequest(err string) (int, interface{}) {
	return GetErrorResponse(http.StatusBadRequest, err)
}

// Unauthorized returns ErrorResponse with code 401
// Usage (with gin):
//      context.JSON(Unauthorized("invalid credentials"))
func Unauthorized(err string) (int, interface{}) {
	return GetErrorResponse(http.StatusUnauthorized, err)
}

// Forbidden returns ErrorResponse with code 403
// Usage (with gin):
//      context.JSON(Forbidden("forbidden"))
func Forbidden(err string) (int, interface{}) {
	return GetErrorResponse(http.StatusForbidden, err)
}

// InternalServerError returns ErrorResponse with code 500
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
func InternalServerError(err string) (int, interface{}) {
	return GetErrorResponse(http.StatusInternalServerError, err)
}
