package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModels_TableName(t *testing.T) {
	assert.NotEmpty(t, (User{}).TableName())
}
