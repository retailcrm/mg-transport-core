package logger

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZabbixCollectorAdapter(t *testing.T) {
	log := newBufferLogger()
	adapter := ZabbixCollectorAdapter(log)
	adapter.Errorf("highly unexpected error: %s", "unexpected error")
	adapter.Errorf("cannot stop collector: %s", "app error")
	adapter.Errorf("cannot send metrics to Zabbix: %v", errors.New("send error"))

	items, err := newJSONBufferedLogger(log).ScanAll()
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "highly unexpected error: unexpected error", items[0].Message)
	assert.Equal(t, "cannot stop Zabbix collector", items[1].Message)
	assert.Equal(t, "app error", items[1].Context[ErrorAttr])
	assert.Equal(t, "cannot send metrics to Zabbix", items[2].Message)
	assert.Equal(t, "send error", items[2].Context[ErrorAttr])
}
