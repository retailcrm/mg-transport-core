package testutil

import (
	"testing"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBufferedLoggerMaskScope(t *testing.T) {
	for _, lazy := range []bool{false, true} {
		t.Run(map[bool]string{false: "With", true: "WithLazy"}[lazy], func(t *testing.T) {
			parent := NewBufferedLoggerSilent()
			with := parent.With

			if lazy {
				with = parent.WithLazy
			}

			with(logger.Mask("token", "api-key")).Info("token api-key", zap.String("key", "api-key"))
			parent.Info("token api-key")

			records, err := NewJSONRecordScanner(parent).ScanAll()
			require.NoError(t, err)
			require.Len(t, records, 2)
			assert.Equal(t, "**** ****", records[0].Message)
			assert.Equal(t, "****", records[0].Context["key"])
			assert.Equal(t, "token api-key", records[1].Message)
		})
	}
}
