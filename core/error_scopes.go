package core

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInsufficientScopes is wrapped by the insufficientScopesErr.
// If errors.Is(err, ErrInsufficientScopes) returns true then error implements ScopesList.
var ErrInsufficientScopes = errors.New("insufficient scopes")

// ScopesList is a contract for the scopes list.
type ScopesList interface {
	Scopes() []string
}

// insufficientScopesErr contains information about missing auth scopes.
type insufficientScopesErr struct {
	scopes  []string
	wrapped error
}

// Error message.
func (e insufficientScopesErr) Error() string {
	return e.wrapped.Error()
}

// Unwrap underlying error.
func (e insufficientScopesErr) Unwrap() error {
	return e.wrapped
}

// Scopes that are missing.
func (e insufficientScopesErr) Scopes() []string {
	return e.scopes
}

// String returns string representation of an error with scopes that are missing.
func (e insufficientScopesErr) String() string {
	return fmt.Sprintf("Missing scopes: %s", strings.Join(e.Scopes(), ", "))
}

// NewInsufficientScopesErr is a insufficientScopesErr constructor.
func NewInsufficientScopesErr(scopes []string) error {
	return insufficientScopesErr{
		scopes:  scopes,
		wrapped: ErrInsufficientScopes,
	}
}
