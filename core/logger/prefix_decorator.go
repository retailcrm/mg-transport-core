package logger

import "github.com/op/go-logging"

// PrefixAware is implemented if the logger allows you to change the prefix.
type PrefixAware interface {
	SetPrefix(string)
}

// PrefixedLogger is a base interface for the logger with prefix.
type PrefixedLogger interface {
	Logger
	PrefixAware
}

// PrefixDecorator is an implementation of the PrefixedLogger. It will allow you to decorate any Logger with
// the provided predefined prefix.
type PrefixDecorator struct {
	backend Logger
	prefix  []interface{}
}

// DecorateWithPrefix using provided base logger and provided prefix.
// No internal state of the base logger will be touched.
func DecorateWithPrefix(backend Logger, prefix string) PrefixedLogger {
	return &PrefixDecorator{backend: backend, prefix: []interface{}{prefix}}
}

// NewWithPrefix returns logger with prefix. It uses StandardLogger under the hood.
func NewWithPrefix(transportCode, prefix string, logLevel logging.Level, logFormat logging.Formatter) PrefixedLogger {
	return DecorateWithPrefix(NewStandard(transportCode, logLevel, logFormat), prefix)
}

// SetPrefix will replace existing prefix with the provided value.
// Use this format for prefixes: "prefix here:" - omit space at the end (it will be inserted automatically).
func (p *PrefixDecorator) SetPrefix(prefix string) {
	p.prefix = []interface{}{prefix}
}

func (p *PrefixDecorator) getFormat(fmt string) string {
	return p.prefix[0].(string) + " " + fmt
}

func (p *PrefixDecorator) Fatal(args ...interface{}) {
	p.backend.Fatal(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Fatalf(format string, args ...interface{}) {
	p.backend.Fatalf(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Panic(args ...interface{}) {
	p.backend.Panic(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Panicf(format string, args ...interface{}) {
	p.backend.Panicf(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Critical(args ...interface{}) {
	p.backend.Critical(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Criticalf(format string, args ...interface{}) {
	p.backend.Criticalf(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Error(args ...interface{}) {
	p.backend.Error(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Errorf(format string, args ...interface{}) {
	p.backend.Errorf(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Warning(args ...interface{}) {
	p.backend.Warning(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Warningf(format string, args ...interface{}) {
	p.backend.Warningf(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Notice(args ...interface{}) {
	p.backend.Notice(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Noticef(format string, args ...interface{}) {
	p.backend.Noticef(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Info(args ...interface{}) {
	p.backend.Info(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Infof(format string, args ...interface{}) {
	p.backend.Infof(p.getFormat(format), args...)
}

func (p *PrefixDecorator) Debug(args ...interface{}) {
	p.backend.Debug(append(p.prefix, args...)...)
}

func (p *PrefixDecorator) Debugf(format string, args ...interface{}) {
	p.backend.Debugf(p.getFormat(format), args...)
}
