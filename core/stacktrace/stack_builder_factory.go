package stacktrace

import "github.com/pkg/errors"

func GetStackBuilderByErrorType(err error) StackBuilderInterface {
	if isPkgErrors(err) {
		return &PkgErrorsBuilder{AbstractStackBuilder{err: err}}
	}

	return &GenericStackBuilder{AbstractStackBuilder{err: err}}
}

func isPkgErrors(err error) bool {
	_, ok := err.(interface {
		StackTrace() errors.StackTrace
	})
	return ok
}
