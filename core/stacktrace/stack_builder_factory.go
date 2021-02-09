package stacktrace

// GetStackBuilderByErrorType tries to guess which stacktrace builder would be feasible for passed error.
// For example, errors from github.com/pkg/errors have StackTrace() method, and Go 1.13 errors can be unwrapped.
func GetStackBuilderByErrorType(err error) StackBuilderInterface {
	if isPkgErrors(err) {
		return &PkgErrorsBuilder{AbstractStackBuilder{err: err}}
	}

	if _, ok := err.(Unwrappable); ok {
		return &UnwrapBuilder{AbstractStackBuilder{err: err}}
	}

	return &GenericStackBuilder{AbstractStackBuilder{err: err}}
}

// isPkgErrors returns true if passed error might be github.com/pkg/errors error.
func isPkgErrors(err error) bool {
	_, okTraceable := err.(PkgErrorTraceable)
	_, okCauseable := err.(PkgErrorCauseable)
	return okTraceable || okCauseable
}
