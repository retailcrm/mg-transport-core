package logger

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateStreamID(t *testing.T) {
	id1 := generateStreamID()
	id2 := generateStreamID()

	assert.NotEqual(t, id1, id2)
}

func BenchmarkGenerateStreamID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = generateStreamID()
	}
}
