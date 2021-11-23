package core

import (
	"github.com/retailcrm/mg-transport-core/core/errortools"
)

// ErrInsufficientScopes is wrapped by the insufficientScopesErr.
// Deprecated: use errortools.ErrInsufficientScopes.
var ErrInsufficientScopes = errortools.ErrInsufficientScopes

// ScopesList is a contract for the scopes list.
// Deprecated: use errortools.ScopesList.
type ScopesList errortools.ScopesList

// NewInsufficientScopesErr is a insufficientScopesErr constructor.
// Deprecated: use errortools.NewInsufficientScopesErr.
func NewInsufficientScopesErr(scopes []string) error {
	return errortools.NewInsufficientScopesErr(scopes)
}
