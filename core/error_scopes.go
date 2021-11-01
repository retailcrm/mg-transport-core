package core

import (
	"fmt"
	"strings"
)

type ErrInsufficientScopes struct{}

func (e *ErrInsufficientScopes) Error() string {
	return "Missing scopes"
}

func NewScopesError(scopes []string) error {
	err := &ErrInsufficientScopes{}
	scopesString := strings.Join(scopes, ", ")
	return fmt.Errorf("%w: %s", err, scopesString)
}
