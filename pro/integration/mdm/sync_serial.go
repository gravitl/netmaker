package mdm

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

func syncHostMDMBySerial(
	ctx context.Context,
	providerID string,
	h schema.Host,
	devices []ManagedDevice,
) error {
	for _, d := range devices {
		if !MatchHostToMDMDeviceBySerial(h, d) {
			continue
		}
		state := schema.DeviceMDMState{
			HostID:       h.ID.String(),
			Provider:     providerID,
			MDMDeviceID:  d.ProviderDeviceID,
			Enrolled:     d.Enrolled,
			Compliant:    d.Compliant,
			MatchedBy:    schema.MDMMatchSerialNumber,
			LastSyncedAt: time.Now().UTC(),
			LastSeenAt:   d.LastSeenAt,
		}
		return state.Upsert(db.WithContext(ctx))
	}
	return upsertUnmatchedHostMDMState(ctx, providerID, h.ID.String(), schema.MDMMatchSerialNumber)
}
