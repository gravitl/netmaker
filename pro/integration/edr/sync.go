package edr

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
	"gorm.io/datatypes"
)

var (
	syncMu   sync.Mutex
	lastSync time.Time
)

func RunEDRSync(ctx context.Context) error {
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

func RunEDRSyncForce(ctx context.Context) error {
	intg, err := GetActive(ctx)
	if err != nil {
		return err
	}
	if intg == nil {
		return errors.New("no EDR integration configured")
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
		logger.Log(0, "edr sync: build provider:", err.Error())
		return err
	}

	endpoints, err := p.ListManagedEndpoints(ctx)
	if err != nil {
		logger.Log(0, "edr sync: list endpoints:", err.Error())
		return err
	}

	hosts, err := (&schema.Host{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(0, "edr sync: list hosts:", err.Error())
		return err
	}

	matched := 0
	for i := range hosts {
		if !hostEligibleForEDR(intg.ID, hosts[i]) {
			if err := clearHostEDRState(ctx, intg.ID, hosts[i].ID.String()); err != nil {
				logger.Log(0, "edr sync: clear stale state for host", hosts[i].ID.String(), ":", err.Error())
			}
			continue
		}
		found := false
		for _, ep := range endpoints {
			matchedBy, ok := MatchHostToEndpoint(intg.ID, hosts[i], ep)
			if !ok {
				continue
			}
			raw := ep.RawVendorData
			if raw == nil {
				raw = json.RawMessage(`{}`)
			}
			state := schema.DeviceEDRState{
				HostID:         hosts[i].ID.String(),
				Provider:       intg.ID,
				EDRDeviceID:    ep.ProviderDeviceID,
				MatchedBy:      matchedBy,
				AgentInstalled: ep.AgentInstalled,
				AgentHealthy:   ep.AgentHealthy,
				RiskLevel:      string(ep.RiskLevel),
				ThreatCount:    ep.ThreatCount,
				ActiveThreats:  ep.ActiveThreats,
				Isolated:       ep.Isolated,
				Contained:      ep.Contained,
				LastSeenAt:     ep.LastSeen,
				LastSyncedAt:   time.Now().UTC(),
				RawVendorData:  datatypes.JSON(raw),
			}
			if err := state.Upsert(db.WithContext(ctx)); err != nil {
				logger.Log(0, "edr sync: upsert state for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			matched++
			found = true
			break
		}
		if !found {
			if err := upsertUnmatchedHostEDRState(ctx, intg.ID, hosts[i].ID.String()); err != nil {
				logger.Log(0, "edr sync: unmatched state for host", hosts[i].ID.String(), ":", err.Error())
			}
		}
	}

	lastSync = time.Now().UTC()
	logger.Log(2, "edr sync: provider=", p.Name(), "endpoints=", itoa(len(endpoints)), "matched=", itoa(matched))
	return nil
}

func hostEligibleForEDR(providerID string, h schema.Host) bool {
	switch providerID {
	case ProviderDefender:
		return strings.TrimSpace(h.EntraDeviceID) != "" || strings.TrimSpace(h.Name) != ""
	default:
		return strings.TrimSpace(h.SerialNumber) != "" || strings.TrimSpace(h.Name) != ""
	}
}

func MatchHostToEndpoint(providerID string, h schema.Host, ep ManagedEndpoint) (matchedBy string, ok bool) {
	switch providerID {
	case ProviderDefender:
		if entra := strings.TrimSpace(h.EntraDeviceID); entra != "" {
			if deviceEntra := strings.TrimSpace(ep.EntraDeviceID); deviceEntra != "" &&
				strings.EqualFold(normalizeGUID(entra), normalizeGUID(deviceEntra)) {
				return schema.EDRMatchEntraDeviceID, true
			}
		}
		if hostnameMatch(h.Name, ep.Hostname) {
			return schema.EDRMatchHostname, true
		}
		return "", false
	default:
		if serialMatch(h.SerialNumber, ep.SerialNumber) {
			return schema.EDRMatchSerialNumber, true
		}
		if hostnameMatch(h.Name, ep.Hostname) {
			return schema.EDRMatchHostname, true
		}
		return "", false
	}
}

func serialMatch(hostSerial, deviceSerial string) bool {
	hostSerial = strings.TrimSpace(hostSerial)
	deviceSerial = strings.TrimSpace(deviceSerial)
	return hostSerial != "" && deviceSerial != "" && strings.EqualFold(hostSerial, deviceSerial)
}

func hostnameMatch(hostName, deviceName string) bool {
	hostName = strings.TrimSpace(strings.ToLower(hostName))
	deviceName = strings.TrimSpace(strings.ToLower(deviceName))
	if hostName == "" || deviceName == "" {
		return false
	}
	hostName = strings.Split(hostName, ".")[0]
	deviceName = strings.Split(deviceName, ".")[0]
	return hostName == deviceName
}

func normalizeGUID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "{")
	id = strings.TrimSuffix(id, "}")
	return id
}

func upsertUnmatchedHostEDRState(ctx context.Context, providerID, hostID string) error {
	state := schema.DeviceEDRState{
		HostID:       hostID,
		Provider:     providerID,
		AgentInstalled: false,
		AgentHealthy:   false,
		RiskLevel:      string(RiskCritical),
		LastSyncedAt:   time.Now().UTC(),
		LastError:      ErrDeviceNotFoundInEDR.Error(),
	}
	return state.Upsert(db.WithContext(ctx))
}

func clearHostEDRState(ctx context.Context, providerID, hostID string) error {
	state := &schema.DeviceEDRState{HostID: hostID, Provider: providerID}
	return state.Delete(db.WithContext(ctx))
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
