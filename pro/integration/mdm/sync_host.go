package mdm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

// RefreshHostMDMState syncs MDM posture state for a single host before join or
// registration posture evaluation. It does not honour the global sync rate limit.
func RefreshHostMDMState(ctx context.Context, h schema.Host) error {
	intg, err := GetActive(ctx)
	if err != nil {
		return err
	}
	if intg == nil {
		return nil
	}
	p, err := Build(intg.ID, json.RawMessage(intg.Config))
	if err != nil {
		return err
	}
	if lookup, ok := p.(EntraDeviceLookup); ok {
		if strings.TrimSpace(h.EntraDeviceID) != "" {
			return upsertHostMDMFromEntraLookup(ctx, intg.ID, lookup, h)
		}
		if strings.TrimSpace(h.SerialNumber) == "" {
			return clearHostMDMState(ctx, intg.ID, h.ID.String())
		}
		devices, err := p.ListManagedDevices(ctx)
		if err != nil {
			return err
		}
		_, err = syncHostMDMBySerial(ctx, intg.ID, h, devices)
		return err
	}
	if strings.TrimSpace(h.SerialNumber) == "" {
		return clearHostMDMState(ctx, intg.ID, h.ID.String())
	}
	devices, err := p.ListManagedDevices(ctx)
	if err != nil {
		return err
	}
	for _, d := range devices {
		if !MatchHostToMDMDeviceBySerial(h, d) {
			continue
		}
		state := schema.DeviceMDMState{
			HostID:       h.ID.String(),
			Provider:     intg.ID,
			MDMDeviceID:  d.ProviderDeviceID,
			Enrolled:     d.Enrolled,
			Compliant:    d.Compliant,
			MatchedBy:    schema.MDMMatchSerialNumber,
			LastSyncedAt: time.Now().UTC(),
			LastSeenAt:   d.LastSeenAt,
		}
		return state.Upsert(db.WithContext(ctx))
	}
	return upsertUnmatchedHostMDMState(ctx, intg.ID, h.ID.String(), schema.MDMMatchSerialNumber)
}
