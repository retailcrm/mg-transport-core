package stacktrace

// GetStackBuilderByErrorType tries to guess which stacktrace builder would be feasible for passed error.
// For example, errors from github.com/pkg/errors have StackTrace() method, and Go 1.13 errors can be unwrapped.
func GetStackBuilderByErrorType(err error) StackBuilderInterface {
	if IsPkgErrorsError(err) {
		return &PkgErrorsBuilder{AbstractStackBuilder{err: err}}
	}

	if IsUnwrappableError(err) {
		return &UnwrapBuilder{AbstractStackBuilder{err: err}}
	}

	if IsErrorNodesList(err) {
		return &ErrCollectorBuilder{AbstractStackBuilder{err: err}}
	}

	return &GenericStackBuilder{AbstractStackBuilder{err: err}}
}
