package mdm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
)

var (
	syncMu   sync.Mutex
	lastSync time.Time
)

// RunMDMSync refreshes DeviceMDMState for hosts via the active provider.
// Intune prefers Entra-keyed lookup and falls back to serial_number when entra_device_id
// is absent; other providers list devices and match serial_number.
// Honours sync_interval_minutes from integration config as an optional per-tick
// rate-limit hint. Returns nil (no-op) if MDM is not configured.
func RunMDMSync(ctx context.Context) error {
	intg, err := GetActive(ctx)
	if err != nil {
		return err
	}
	if intg == nil {
		return nil
	}
	sync, err := ParseSyncSettings(intg.ID, json.RawMessage(intg.Config))
	if err != nil {
		return err
	}
	if !sync.SyncEnabled {
		return nil
	}
	return runSyncLocked(ctx, intg, false)
}

// RunMDMSyncForce ignores the rate-limit hint and triggers a fresh sync.
func RunMDMSyncForce(ctx context.Context) error {
	intg, err := GetActive(ctx)
	if err != nil {
		return err
	}
	if intg == nil {
		return errors.New("no MDM integration configured")
	}
	return runSyncLocked(ctx, intg, true)
}

func runSyncLocked(ctx context.Context, intg *schema.Integration, force bool) error {
	syncMu.Lock()
	defer syncMu.Unlock()

	sync, err := ParseSyncSettings(intg.ID, json.RawMessage(intg.Config))
	if err != nil {
		return err
	}
	if !force && sync.SyncIntervalMinutes > 0 &&
		!lastSync.IsZero() &&
		time.Since(lastSync) < time.Duration(sync.SyncIntervalMinutes)*time.Minute {
		return nil
	}

	p, err := Build(intg.ID, json.RawMessage(intg.Config))
	if err != nil {
		logger.Log(0, "mdm sync: build provider:", err.Error())
		return err
	}

	hosts, err := (&schema.Host{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(0, "mdm sync: list hosts:", err.Error())
		return err
	}

	matched := 0
	if lookup, ok := p.(EntraDeviceLookup); ok {
		var devices []ManagedDevice
		devicesLoaded := false
		for i := range hosts {
			if strings.TrimSpace(hosts[i].EntraDeviceID) != "" {
				if err := upsertHostMDMFromEntraLookup(ctx, intg.ID, lookup, hosts[i]); err != nil {
					logger.Log(0, "mdm sync: entra lookup for host", hosts[i].ID.String(), ":", err.Error())
					continue
				}
				matched++
				continue
			}
			if strings.TrimSpace(hosts[i].SerialNumber) == "" {
				if err := clearHostMDMState(ctx, intg.ID, hosts[i].ID.String()); err != nil {
					logger.Log(0, "mdm sync: clear stale state for host", hosts[i].ID.String(), ":", err.Error())
				}
				continue
			}
			if !devicesLoaded {
				var err error
				devices, err = p.ListManagedDevices(ctx)
				if err != nil {
					logger.Log(0, "mdm sync: list devices:", err.Error())
					return err
				}
				devicesLoaded = true
			}
			ok, err := syncHostMDMBySerial(ctx, intg.ID, hosts[i], devices)
			if err != nil {
				logger.Log(0, "mdm sync: serial match for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			if ok {
				matched++
			}
		}
		lastSync = time.Now().UTC()
		logger.Log(2, "mdm sync: provider=", p.Name(), "matched=", itoa(matched))
		return nil
	}

	devices, err := p.ListManagedDevices(ctx)
	if err != nil {
		logger.Log(0, "mdm sync: list devices:", err.Error())
		return err
	}
	for i := range hosts {
		if strings.TrimSpace(hosts[i].SerialNumber) == "" {
			if err := clearHostMDMState(ctx, intg.ID, hosts[i].ID.String()); err != nil {
				logger.Log(0, "mdm sync: clear stale state for host", hosts[i].ID.String(), ":", err.Error())
			}
			continue
		}
		found := false
		for _, d := range devices {
			if !MatchHostToMDMDeviceBySerial(hosts[i], d) {
				continue
			}
			state := schema.DeviceMDMState{
				HostID:       hosts[i].ID.String(),
				Provider:     intg.ID,
				MDMDeviceID:  d.ProviderDeviceID,
				Enrolled:     d.Enrolled,
				Compliant:    d.Compliant,
				MatchedBy:    schema.MDMMatchSerialNumber,
				LastSyncedAt: time.Now().UTC(),
				LastSeenAt:   d.LastSeenAt,
			}
			if err := state.Upsert(db.WithContext(ctx)); err != nil {
				logger.Log(0, "mdm sync: upsert state for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			matched++
			found = true
			break
		}
		if !found {
			if err := upsertUnmatchedHostMDMState(ctx, intg.ID, hosts[i].ID.String(), schema.MDMMatchSerialNumber); err != nil {
				logger.Log(0, "mdm sync: clear state for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
		}
	}

	lastSync = time.Now().UTC()
	logger.Log(2, "mdm sync: provider=", p.Name(), "devices=", itoa(len(devices)), "matched=", itoa(matched))
	return nil
}

func upsertUnmatchedHostMDMState(ctx context.Context, providerID, hostID, matchedBy string) error {
	state := schema.DeviceMDMState{
		HostID:       hostID,
		Provider:     providerID,
		Enrolled:     false,
		Compliant:    false,
		MatchedBy:    matchedBy,
		LastSyncedAt: time.Now().UTC(),
		LastError:    ErrDeviceNotFoundInMDM.Error(),
	}
	return state.Upsert(db.WithContext(ctx))
}

func clearHostMDMState(ctx context.Context, providerID, hostID string) error {
	state := &schema.DeviceMDMState{HostID: hostID, Provider: providerID}
	return state.Delete(db.WithContext(ctx))
}

// MatchHostToMDMDeviceBySerial matches a host to an MDM device by serial number only.
func MatchHostToMDMDeviceBySerial(h schema.Host, d ManagedDevice) bool {
	hostSerial := strings.TrimSpace(h.SerialNumber)
	deviceSerial := strings.TrimSpace(d.SerialNumber)
	return hostSerial != "" && deviceSerial != "" &&
		strings.EqualFold(hostSerial, deviceSerial)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
