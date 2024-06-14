package errorutil

import (
	"fmt"
	"runtime"
	"strings"
)

// Collector is a replacement for the core.ErrorCollector function. It is easier to use and contains more functionality.
// For example, you can iterate over the errors or use Collector.Panic() to immediately panic
// if there are errors in the chain.
//
// Error messages will differ from the ones produced by ErrorCollector. However, it's for the best because
// new error messages contain a lot more useful information and can be even used as a stacktrace.
//
// Collector implements Error() and String() methods. As a result, you can use the collector as an error itself or just
// print out it as a value. However, it is better to use AsError() method if you want to use Collector as an error value
// because AsError() returns nil if there are no errors in the list.
//
// Example:
//
//	err := errorutil.NewCollector().
//	            Do(errors.New("error 1")).
//	            Do(errors.New("error 2"), errors.New("error 3"))
//	// Will print error message.
//	fmt.Println(err)
//
// This code will produce something like this:
//
//	#1 err at /home/user/main.go:62: error 1
//	#2 err at /home/user/main.go:63: error 2
//	#3 err at /home/user/main.go:64: error 3
//
// You can also iterate over the error to use their data instead of using predefined message:
//
//	err := errorutil.NewCollector().
//	            Do(errors.New("error 1")).
//	            Do(errors.New("error 2"), errors.New("error 3"))
//
//	for err := range c.Iterate() {
//		fmt.Printf("Error at %s:%d: %v\n", err.File, err.Line, err)
//	}
//
// This code will produce output that looks like this:
//
//	Error at /home/user/main.go:164: error 0
//	Error at /home/user/main.go:164: error 1
//	Error at /home/user/main.go:164: error 2
//
// Example with GORM migration (Collector is returned as an error here).
//
//	return errorutil.NewCollector().Do(
//	    db.CreateTable(models.Account{}, models.Connection{}).Error,
//	    db.Table("account").AddUniqueIndex("account_key", "channel").Error,
//	).AsError()
type Collector struct {
	errors *errList
}

// NewCollector returns new errorutil.Collector instance.
func NewCollector() *Collector {
	return &Collector{
		errors: &errList{},
	}
}

// Collect errors, return one error for all of them (shorthand for errorutil.NewCollector().Do(...).AsError()).
// Returns nil if there are no errors.
func Collect(errs ...error) error {
	return NewCollector().Do(errs...).AsError()
}

// Do some operation that returns the error. Supports multiple operations at once.
func (e *Collector) Do(errs ...error) *Collector {
	pc, file, line, _ := runtime.Caller(1)

	for _, err := range errs {
		if err != nil {
			e.errors.Push(pc, err, file, line)
		}
	}

	return e
}

// OK returns true if there is no errors in the list.
func (e *Collector) OK() bool {
	return e.errors.Len() == 0
}

// Error message.
func (e *Collector) Error() string {
	return e.buildErrorMessage()
}

// AsError returns the Collector itself as an error, but only if there are errors in the list.
// It returns nil otherwise. This method should be used if you want to return error to the caller, but only if\
// Collector actually caught something.
func (e *Collector) AsError() error {
	if e.OK() {
		return nil
	}
	return e
}

// String with an error message.
func (e *Collector) String() string {
	return e.Error()
}

// Panic with the error data if there are errors in the list.
func (e *Collector) Panic() {
	if !e.OK() {
		panic(e)
	}
}

// Iterate over the errors in the list. Every error is represented as an errorutil.Node value.
func (e *Collector) Iterate() <-chan Node {
	return e.errors.Iterate()
}

// Len returns the number of the errors in the list.
func (e *Collector) Len() int {
	return e.errors.Len()
}

// buildErrorMessage builds error message for the Collector.Error() and Collector.String() methods.
func (e *Collector) buildErrorMessage() string {
	i := 0
	var sb strings.Builder
	sb.Grow(128 * e.errors.Len()) // nolint:gomnd

	for node := range e.errors.Iterate() {
		i++
		sb.WriteString(fmt.Sprintf("#%d err at %s:%d: %v\n", i, node.File, node.Line, node.Err))
	}

	return strings.TrimRight(sb.String(), "\n")
}
