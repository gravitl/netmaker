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

// RunMDMSync pulls the latest managed-device snapshot from the configured MDM
// provider and upserts DeviceMDMState rows for every host that matches.
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

	devices, err := p.ListManagedDevices(ctx)
	if err != nil {
		logger.Log(0, "mdm sync: list devices:", err.Error())
		return err
	}

	hosts, err := (&schema.Host{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(0, "mdm sync: list hosts:", err.Error())
		return err
	}

	matched := 0
	for _, d := range devices {
		for i := range hosts {
			ok, by := MatchHostToMDMDevice(hosts[i], d)
			if !ok {
				continue
			}
			state := schema.DeviceMDMState{
				HostID:       hosts[i].ID.String(),
				Provider:     intg.ID,
				MDMDeviceID:  d.ProviderDeviceID,
				Enrolled:     d.Enrolled,
				Compliant:    d.Compliant,
				MatchedBy:    by,
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

// MatchHostToMDMDevice walks the matching ladder:
// EntraDeviceID -> SerialNumber -> HardwareUUID -> Hostname+Email -> Hostname.
func MatchHostToMDMDevice(h schema.Host, d ManagedDevice) (matched bool, by string) {
	if h.EntraDeviceID != "" && d.AzureADDeviceID != "" &&
		strings.EqualFold(h.EntraDeviceID, d.AzureADDeviceID) {
		return true, schema.MDMMatchEntraDeviceID
	}
	if h.SerialNumber != "" && d.SerialNumber != "" &&
		strings.EqualFold(h.SerialNumber, d.SerialNumber) {
		return true, schema.MDMMatchSerialNumber
	}
	if h.HardwareUUID != "" && d.HardwareUUID != "" &&
		strings.EqualFold(h.HardwareUUID, d.HardwareUUID) {
		return true, schema.MDMMatchHardwareUUID
	}
	if h.Name != "" && d.DeviceName != "" &&
		hostNamesMatch(h.Name, d.DeviceName) &&
		h.UserEmail != "" && d.UserPrincipalName != "" &&
		strings.EqualFold(h.UserEmail, d.UserPrincipalName) {
		return true, schema.MDMMatchHostnameEmail
	}
	if h.Name != "" && d.DeviceName != "" && hostNamesMatch(h.Name, d.DeviceName) {
		return true, schema.MDMMatchHostname
	}
	return false, ""
}

// hostNamesMatch compares host and MDM device names, treating FQDN and short
// hostname as equivalent (e.g. "laptop" vs "laptop.corp.example.com").
func hostNamesMatch(hostName, deviceName string) bool {
	hostName = strings.TrimSpace(hostName)
	deviceName = strings.TrimSpace(deviceName)
	if hostName == "" || deviceName == "" {
		return false
	}
	if strings.EqualFold(hostName, deviceName) {
		return true
	}
	return strings.EqualFold(shortHostname(hostName), shortHostname(deviceName))
}

func shortHostname(name string) string {
	if i := strings.Index(name, "."); i > 0 {
		return name[:i]
	}
	return name
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
