package edr

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

// SyncHostEDRState refreshes EDR posture state for one host from the active
// provider. Called after check-in when device-matching identifiers change.
func SyncHostEDRState(ctx context.Context, hostID string) error {
	intg, err := GetActive(ctx)
	if err != nil {
		return err
	}
	if intg == nil {
		return nil
	}
	id, err := uuid.Parse(hostID)
	if err != nil {
		return err
	}
	h := &schema.Host{ID: id}
	if err := h.Get(db.WithContext(ctx)); err != nil {
		return err
	}
	if !hostEligibleForEDR(intg.ID, *h) {
		return nil
	}
	state := &schema.DeviceEDRState{HostID: hostID, Provider: intg.ID}
	if err := state.Get(db.WithContext(ctx)); err == nil &&
		time.Since(state.LastSyncedAt) < 5*time.Minute {
		return nil
	}
	return RefreshHostEDRState(ctx, *h)
}
