package logger

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
)

func TestWriterAdapter(t *testing.T) {
	log := newBufferLogger()
	adapter := WriterAdapter(log, zap.InfoLevel)

	msg := []byte("hello world")
	total, err := adapter.Write(msg)
	require.NoError(t, err)
	require.Equal(t, total, len(msg))

	items, err := newJSONBufferedLogger(log).ScanAll()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "hello world", items[0].Message)
	assert.Equal(t, "INFO", items[0].LevelName)
}
