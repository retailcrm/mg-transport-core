package beanstalk

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"time"
)

// LegacyBodyAdapter lets a Driver consume jobs written before queue envelopes were introduced.
// It passes valid envelopes through and wraps all other bodies as legacy payloads. Use a codec
// that can decode the original job body, and remove this adapter after the old tube is drained.
type LegacyBodyAdapter struct {
	ManagerInterface
}

// NewLegacyBodyAdapter wraps a manager for a tube containing old, unwrapped jobs.
func NewLegacyBodyAdapter(manager ManagerInterface) *LegacyBodyAdapter {
	return &LegacyBodyAdapter{ManagerInterface: manager}
}

// PutContext forwards context-aware puts when supported by the wrapped manager.
func (m *LegacyBodyAdapter) PutContext(
	ctx context.Context, body []byte, priority uint32, delay, ttr time.Duration,
) (uint64, error) {
	if manager, ok := m.ManagerInterface.(interface {
		PutContext(context.Context, []byte, uint32, time.Duration, time.Duration) (uint64, error)
	}); ok {
		return manager.PutContext(ctx, body, priority, delay, ttr)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return m.ManagerInterface.Put(body, priority, delay, ttr)
}

// Reserve implements ManagerInterface while preserving the original job body for the codec.
func (m *LegacyBodyAdapter) Reserve(timeout time.Duration) (uint64, []byte, error) {
	id, body, err := m.ManagerInterface.Reserve(timeout)
	if err != nil {
		return id, body, err
	}
	var fields struct {
		ID         string    `json:"id"`
		EnqueuedAt time.Time `json:"enqueuedAt"`
		Payload    []byte    `json:"payload"`
	}
	if json.Unmarshal(body, &fields) == nil && fields.ID != "" &&
		!fields.EnqueuedAt.IsZero() && fields.Payload != nil {
		return id, body, nil
	}
	wrapped, err := json.Marshal(struct {
		ID         string    `json:"id"`
		EnqueuedAt time.Time `json:"enqueuedAt"`
		Payload    []byte    `json:"payload"`
	}{ID: fmt.Sprintf("legacy-%d", id), EnqueuedAt: time.Now(), Payload: body})
	if err != nil {
		return id, nil, err
	}
	return id, wrapped, nil
}

var _ ManagerInterface = (*LegacyBodyAdapter)(nil)
