package logic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/mdm"
	"github.com/gravitl/netmaker/schema"
)

var (
	mdmSyncMu   sync.Mutex
	lastMDMSync time.Time
)

// RunMDMSync pulls the latest managed-device snapshot from the configured MDM
// provider and upserts DeviceMDMState rows for every host that matches.
// Honours ServerSettings.MDMSyncIntervalMinutes as an optional per-tick
// rate-limit hint. Returns nil (no-op) if MDM is not configured.
func RunMDMSync(ctx context.Context) error {
	s := logic.GetServerSettings()
	if !s.MDMSyncEnabled || s.MDMProvider == "" {
		return nil
	}
	mdmSyncMu.Lock()
	if s.MDMSyncIntervalMinutes > 0 &&
		!lastMDMSync.IsZero() &&
		time.Since(lastMDMSync) < time.Duration(s.MDMSyncIntervalMinutes)*time.Minute {
		mdmSyncMu.Unlock()
		return nil
	}
	mdmSyncMu.Unlock()
	return runMDMSyncLocked(ctx, s)
}

// RunMDMSyncForce ignores the rate-limit hint and triggers a fresh sync. Used
// by the admin "Sync now" endpoint.
func RunMDMSyncForce(ctx context.Context) error {
	s := logic.GetServerSettings()
	if s.MDMProvider == "" {
		return errors.New("no MDM provider configured")
	}
	return runMDMSyncLocked(ctx, s)
}

func runMDMSyncLocked(ctx context.Context, s models.ServerSettings) error {
	mdmSyncMu.Lock()
	defer mdmSyncMu.Unlock()

	p, err := mdm.BuildActive(s)
	if err != nil {
		logger.Log(0, "mdm sync: build provider:", err.Error())
		return err
	}
	if p == nil {
		return nil
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
				Provider:     p.Name(),
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
	lastMDMSync = time.Now().UTC()
	logger.Log(2, "mdm sync: provider=", p.Name(), "devices=", itoa(len(devices)), "matched=", itoa(matched))
	return nil
}

// MatchHostToMDMDevice walks the matching ladder defined in the plan:
// EntraDeviceID -> SerialNumber -> HardwareUUID -> Hostname+Email -> Hostname.
// Returns (true, reason) on the first match.
func MatchHostToMDMDevice(h schema.Host, d mdm.ManagedDevice) (matched bool, by string) {
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
		strings.EqualFold(h.Name, d.DeviceName) &&
		h.UserEmail != "" && d.UserPrincipalName != "" &&
		strings.EqualFold(h.UserEmail, d.UserPrincipalName) {
		return true, schema.MDMMatchHostnameEmail
	}
	if h.Name != "" && d.DeviceName != "" &&
		strings.EqualFold(h.Name, d.DeviceName) {
		return true, schema.MDMMatchHostname
	}
	return false, ""
}

func itoa(i int) string {
	// Minimal local helper to avoid pulling strconv just for log lines.
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
