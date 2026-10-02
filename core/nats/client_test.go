package nats

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthValidation(t *testing.T) {
	_, err := authOption(Auth{Username: "user", Token: "token"})
	require.Error(t, err)
	_, err = authOption(Auth{Password: "password"})
	require.Error(t, err)
	option, err := authOption(Auth{Token: "token"})
	require.NoError(t, err)
	require.NotNil(t, option)
}
