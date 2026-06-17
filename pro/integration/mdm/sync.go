package mdm

import (
	"context"
	"encoding/json"
	"errors"
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

// RunMDMSync refreshes DeviceMDMState for hosts with entra_device_id via the
// active provider's Entra-keyed lookup. Intune calls Graph /devices first;
// managedDevices is only queried when /devices returns no match.
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

	lookup, ok := p.(EntraDeviceLookup)
	if !ok {
		logger.Log(2, "mdm sync: provider=", p.Name(), "does not support entra device lookup, skipping")
		lastSync = time.Now().UTC()
		return nil
	}

	hosts, err := (&schema.Host{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(0, "mdm sync: list hosts:", err.Error())
		return err
	}

	matched := 0
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
