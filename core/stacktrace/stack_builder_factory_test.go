package stacktrace

import (
	"errors"
	"testing"

	pkgErrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/retailcrm/mg-transport-core/core/errorutil"
)

func TestGetStackBuilderByErrorType_PkgErrors(t *testing.T) {
	testErr := pkgErrors.New("pkg/errors err")
	builder := GetStackBuilderByErrorType(testErr)
	assert.IsType(t, &PkgErrorsBuilder{}, builder)
}

func TestGetStackBuilderByErrorType_UnwrapBuilder(t *testing.T) {
	testErr := newWrappableError("first", newWrappableError("second", errors.New("third")))
	builder := GetStackBuilderByErrorType(testErr)
	assert.IsType(t, &UnwrapBuilder{}, builder)
}

func TestGetStackBuilderByErrorType_ErrCollectorBuilder(t *testing.T) {
	testErr := errorutil.NewCollector().Do(errors.New("first"), errors.New("second")).AsError()
	builder := GetStackBuilderByErrorType(testErr)
	assert.IsType(t, &ErrCollectorBuilder{}, builder)
}

func TestGetStackBuilderByErrorType_Generic(t *testing.T) {
	defaultErr := errors.New("default err")
	builder := GetStackBuilderByErrorType(defaultErr)
	assert.IsType(t, &GenericStackBuilder{}, builder)
}
