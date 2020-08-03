package core

import "net/http"

// ErrorResponse struct
type ErrorResponse struct {
	Error string `json:"error"`
}

// ErrorsResponse struct
type ErrorsResponse struct {
	Error []string `json:"error"`
}

// GetErrorResponse returns ErrorResponse with specified status code
// Usage (with gin):
//      context.JSON(GetErrorResponse(http.StatusPaymentRequired, "Not enough money"))
func GetErrorResponse(statusCode int, error string) (int, interface{}) {
	return statusCode, ErrorResponse{
		Error: error,
	}
}

// BadRequest returns ErrorResponse with code 400
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
func BadRequest(error string) (int, interface{}) {
	return GetErrorResponse(http.StatusBadRequest, error)
}

// InternalServerError returns ErrorResponse with code 500
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
func InternalServerError(error string) (int, interface{}) {
	return GetErrorResponse(http.StatusInternalServerError, error)
}
