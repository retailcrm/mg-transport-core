package errorutil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_GetErrorResponse(t *testing.T) {
	code, resp := GetErrorResponse(http.StatusBadRequest, "error string")

	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "error string", resp.(Response).Error)
}

func TestError_BadRequest(t *testing.T) {
	code, resp := BadRequest("error string")

	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "error string", resp.(Response).Error)
}

func TestError_InternalServerError(t *testing.T) {
	code, resp := InternalServerError("error string")

	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error string", resp.(Response).Error)
}
