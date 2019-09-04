package core

import "net/http"

// ErrorResponse struct
type ErrorResponse struct {
	Error string `json:"error"`
}

// BadRequest returns ErrorResponse with code 400
// Usage (with gin):
//      context.JSON(BadRequest("invalid data"))
func BadRequest(error string) (int, interface{}) {
	return http.StatusBadRequest, ErrorResponse{
		Error: error,
	}
}
