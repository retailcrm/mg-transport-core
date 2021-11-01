package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestError_NewScopesError(t *testing.T) {
	scopes := []string{"scope1", "scope2"}
	scopesError := NewScopesError(scopes)

	assert.Equal(t, scopesError.Error(), "Missing scopes: scope1, scope2")
}
