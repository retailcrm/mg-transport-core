package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCollector_NoError(t *testing.T) {
	err := ErrorCollector(nil, nil, nil)

	assert.NoError(t, err)
	assert.Nil(t, err)
}

func TestErrorCollector_SeveralErrors(t *testing.T) {
	err := ErrorCollector(nil, errors.New("error text"), nil)

	assert.Error(t, err)
	assert.Equal(t, "error text", err.Error())
}

func TestErrorCollector_EmptyErrorMessage(t *testing.T) {
	err := ErrorCollector(nil, errors.New(""), nil)

	assert.Error(t, err)
	assert.Equal(t, "", err.Error())
}

func TestErrorCollector_AllErrors(t *testing.T) {
	err := ErrorCollector(
		errors.New("first"),
		errors.New("second"),
		errors.New("third"),
	)

	assert.Error(t, err)
	assert.Equal(t, "first < second < third", err.Error())
}
