package logger

import (
	"fmt"

	"github.com/op/go-logging"
)

// DefaultAccountLoggerFormat contains default prefix format for the AccountLoggerDecorator.
// Its messages will look like this (assuming you will provide the connection URL and account name):
//      messageHandler (https://any.simla.com => @tg_account): sent message with id=1
const DefaultAccountLoggerFormat = "%s (%s => %s):"

type ComponentAware interface {
	SetComponent(string)
}

type ConnectionAware interface {
	SetConnectionIdentifier(string)
}

type AccountAware interface {
	SetAccountIdentifier(string)
}

type PrefixFormatAware interface {
	SetPrefixFormat(string)
}

type AccountLogger interface {
	PrefixedLogger
	ComponentAware
	ConnectionAware
	AccountAware
	PrefixFormatAware
}

type AccountLoggerDecorator struct {
	format         string
	component      string
	connIdentifier string
	accIdentifier  string
	PrefixDecorator
}

func DecorateForAccount(base Logger, component, connIdentifier, accIdentifier string) AccountLogger {
	return (&AccountLoggerDecorator{
		PrefixDecorator: PrefixDecorator{
			backend: base,
		},
		component:      component,
		connIdentifier: connIdentifier,
		accIdentifier:  accIdentifier,
	}).updatePrefix()
}

// NewForAccount returns logger for account. It uses StandardLogger under the hood.
func NewForAccount(
	transportCode, component, connIdentifier, accIdentifier string,
	logLevel logging.Level,
	logFormat logging.Formatter) AccountLogger {
	return DecorateForAccount(NewStandard(transportCode, logLevel, logFormat),
		component, connIdentifier, accIdentifier)
}

func (a *AccountLoggerDecorator) SetComponent(s string) {
	a.component = s
	a.updatePrefix()
}

func (a *AccountLoggerDecorator) SetConnectionIdentifier(s string) {
	a.connIdentifier = s
	a.updatePrefix()
}

func (a *AccountLoggerDecorator) SetAccountIdentifier(s string) {
	a.accIdentifier = s
	a.updatePrefix()
}

func (a *AccountLoggerDecorator) SetPrefixFormat(s string) {
	a.format = s
	a.updatePrefix()
}

func (a *AccountLoggerDecorator) updatePrefix() AccountLogger {
	a.SetPrefix(fmt.Sprintf(a.prefixFormat(), a.component, a.connIdentifier, a.accIdentifier))
	return a
}

func (a *AccountLoggerDecorator) prefixFormat() string {
	if a.format == "" {
		return DefaultAccountLoggerFormat
	}
	return a.format
}
