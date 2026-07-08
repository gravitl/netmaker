package mdm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
)

// EntraDeviceLookup is implemented by MDM providers that resolve a host using
// host.entra_device_id as Graph devices.deviceId. Intune queries GET /v1.0/devices
// first, then GET /deviceManagement/managedDevices when /devices returns no match.
// When entra_device_id is absent, Intune falls back to serial_number matching via
// ListManagedDevices.
type EntraDeviceLookup interface {
	LookupByEntraDeviceID(ctx context.Context, entraDeviceID string) (ManagedDevice, error)
}

// SyncHostMDMState refreshes MDM posture state for one host. When the active
// provider supports Entra-keyed lookup, Graph is queried by entra_device_id or,
// when that is absent, the host is matched by serial_number.
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
	if strings.TrimSpace(h.EntraDeviceID) != "" {
		return upsertHostMDMFromEntraLookup(ctx, intg.ID, lookup, *h)
	}
	if strings.TrimSpace(h.SerialNumber) == "" {
		return nil
	}
	devices, err := p.ListManagedDevices(ctx)
	if err != nil {
		return err
	}
	_, err = syncHostMDMBySerial(ctx, intg.ID, *h, devices)
	return err
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
