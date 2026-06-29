package mdm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
)

// EntraDeviceLookup is implemented by MDM providers that resolve a host using
// host.entra_device_id as Graph devices.deviceId. Intune queries GET /v1.0/devices
// first, then GET /deviceManagement/managedDevices when /devices returns no match.
//
// Providers without EntraDeviceLookup (Iru, Jamf, JumpCloud) are synced via
// ListManagedDevices and serial_number matching in sync.go.
type EntraDeviceLookup interface {
	LookupByEntraDeviceID(ctx context.Context, entraDeviceID string) (ManagedDevice, error)
}

// SyncHostMDMState refreshes MDM posture state for one host. When the host has
// entra_device_id and the active provider supports Entra-keyed lookup, Graph is
// queried directly; otherwise this is a no-op.
func SyncHostMDMState(ctx context.Context, hostID string) error {
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
	lookup, ok := p.(EntraDeviceLookup)
	if !ok {
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
	if h.EntraDeviceID == "" {
		return nil
	}
	return upsertHostMDMFromEntraLookup(ctx, intg.ID, lookup, *h)
}

func upsertHostMDMFromEntraLookup(
	ctx context.Context,
	providerID string,
	lookup EntraDeviceLookup,
	h schema.Host,
) error {
	device, err := lookup.LookupByEntraDeviceID(ctx, h.EntraDeviceID)
	now := time.Now().UTC()
	state := schema.DeviceMDMState{
		HostID:       h.ID.String(),
		Provider:     providerID,
		MatchedBy:    schema.MDMMatchEntraDeviceID,
		LastSyncedAt: now,
	}
	if code := LookupErrorCode(err); code != "" {
		state.LastError = code
		state.Enrolled = false
		state.Compliant = false
		if upsertErr := state.Upsert(db.WithContext(ctx)); upsertErr != nil {
			return upsertErr
		}
		return nil
	}
	if err != nil {
		return err
	}
	state.MDMDeviceID = device.ProviderDeviceID
	state.Enrolled = device.Enrolled
	state.Compliant = device.Compliant
	state.LastSeenAt = device.LastSeenAt
	state.LastError = ""
	if upsertErr := state.Upsert(db.WithContext(ctx)); upsertErr != nil {
		return upsertErr
	}
	logger.Log(2, "mdm sync: entra lookup matched host", h.ID.String(), "device", device.ProviderDeviceID)
	return nil
}
