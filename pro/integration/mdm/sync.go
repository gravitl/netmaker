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
// Intune uses Entra-keyed lookup; other providers list devices and match serial_number.
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
	syncMu.Lock()
	if sync.SyncIntervalMinutes > 0 &&
		!lastSync.IsZero() &&
		time.Since(lastSync) < time.Duration(sync.SyncIntervalMinutes)*time.Minute {
		syncMu.Unlock()
		return nil
	}
	syncMu.Unlock()
	return runSyncLocked(ctx, intg)
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
	return runSyncLocked(ctx, intg)
}

func runSyncLocked(ctx context.Context, intg *schema.Integration) error {
	syncMu.Lock()
	defer syncMu.Unlock()

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
		for i := range hosts {
			if hosts[i].EntraDeviceID == "" {
				continue
			}
			if err := upsertHostMDMFromEntraLookup(ctx, intg.ID, lookup, hosts[i]); err != nil {
				logger.Log(0, "mdm sync: entra lookup for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			matched++
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
			continue
		}
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
			break
		}
	}

	lastSync = time.Now().UTC()
	logger.Log(2, "mdm sync: provider=", p.Name(), "devices=", itoa(len(devices)), "matched=", itoa(matched))
	return nil
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
