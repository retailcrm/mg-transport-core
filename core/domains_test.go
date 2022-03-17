package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetSaasDomains(t *testing.T) {
	domains := GetSaasDomains()

	if domains == nil {
		t.Fail()
	}

	assert.NotEmpty(t, domains)
}

func Test_GetBoxDomains(t *testing.T) {
	domains := GetBoxDomains()

	if domains == nil {
		t.Fail()
	}

	assert.NotEmpty(t, domains)
}
