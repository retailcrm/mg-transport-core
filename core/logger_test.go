package core

import (
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
)

func TestLogger_NewLogger(t *testing.T) {
	logger := NewLogger("code", logging.DEBUG, DefaultLogFormatter())

	assert.NotNil(t, logger)
}

func TestLogger_DefaultLogFormatter(t *testing.T) {
	formatter := DefaultLogFormatter()

	assert.NotNil(t, formatter)
	assert.IsType(t, logging.MustStringFormatter(`%{message}`), formatter)
}
