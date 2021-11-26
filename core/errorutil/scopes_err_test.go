package errorutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_NewScopesError(t *testing.T) {
	scopes := []string{"scope1", "scope2"}
	scopesError := NewInsufficientScopesErr(scopes)

	assert.True(t, errors.Is(scopesError, ErrInsufficientScopes))
	assert.Equal(t, scopes, AsInsufficientScopesErr(scopesError).Scopes())
}
