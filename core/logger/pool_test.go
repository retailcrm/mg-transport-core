package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool(t *testing.T) {
	p := NewPool[*uint8](func() *uint8 {
		item := uint8(22)
		return &item
	})

	val := p.Get()
	assert.Equal(t, uint8(22), *val)
	assert.Equal(t, uint8(22), *p.Get())
	p.Put(val)
}
