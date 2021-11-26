package logger

import (
	"fmt"
	"os"
)

// Nil provides Logger implementation that does almost nothing when called.
// Panic, Panicf, Fatal and Fatalf methods still cause panic and immediate program termination respectively.
// All other methods won't do anything at all.
type Nil struct{}

func (n Nil) Fatal(args ...interface{}) {
	os.Exit(1)
}

func (n Nil) Fatalf(format string, args ...interface{}) {
	os.Exit(1)
}

func (n Nil) Panic(args ...interface{}) {
	panic(fmt.Sprint(args...))
}

func (n Nil) Panicf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

func (n Nil) Critical(args ...interface{})                 {}
func (n Nil) Criticalf(format string, args ...interface{}) {}
func (n Nil) Error(args ...interface{})                    {}
func (n Nil) Errorf(format string, args ...interface{})    {}
func (n Nil) Warning(args ...interface{})                  {}
func (n Nil) Warningf(format string, args ...interface{})  {}
func (n Nil) Notice(args ...interface{})                   {}
func (n Nil) Noticef(format string, args ...interface{})   {}
func (n Nil) Info(args ...interface{})                     {}
func (n Nil) Infof(format string, args ...interface{})     {}
func (n Nil) Debug(args ...interface{})                    {}
func (n Nil) Debugf(format string, args ...interface{})    {}

// NewNil is a Nil logger constructor.
func NewNil() Logger {
	return &Nil{}
}
