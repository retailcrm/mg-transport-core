package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTool_MigrationCommandExists(t *testing.T) {
	found := false

	for _, cmd := range parser.Commands() {
		if cmd != nil && cmd.Name == "migration" {
			found = true
		}
	}

	assert.True(t, found)
}
