package memory

import (
	"context"
	"testing"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/require"
)

func TestDequeueReadyItemAfterCancellationOrClose(t *testing.T) {
	for _, test := range []struct {
		name   string
		cancel bool
	}{
		{name: "canceled context", cancel: true},
		{name: "closed driver"},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := New[int](Options{})
			require.NoError(t, driver.Enqueue(t.Context(), 42, queue.EnqueueOptions{}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.cancel {
				cancel()
			} else {
				require.NoError(t, driver.Close(t.Context()))
			}
			delivery, err := driver.Dequeue(ctx)
			require.Nil(t, delivery)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}
