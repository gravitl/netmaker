package edr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
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
	sync, err := ParseSyncSettings(intg.Provider, json.RawMessage(intg.Config))
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

// RunEDRSyncForPosture runs a full EDR sync before the posture evaluation cycle,
// ignoring sync_enabled and the rate limit.
func RunEDRSyncForPosture(ctx context.Context) error {
	intg, err := GetActive(ctx)
	if err != nil || intg == nil {
		return nil
	}
	return runSyncLocked(ctx, intg, true)
}

func runSyncLocked(ctx context.Context, intg *schema.Integration, force bool) error {
	syncMu.Lock()
	defer syncMu.Unlock()

	sync, err := ParseSyncSettings(intg.Provider, json.RawMessage(intg.Config))
	if err != nil {
		return err
	}
	if !force && sync.SyncIntervalMinutes > 0 &&
		!lastSync.IsZero() &&
		time.Since(lastSync) < time.Duration(sync.SyncIntervalMinutes)*time.Minute {
		return nil
	}

	p, err := Build(intg.Provider, json.RawMessage(intg.Config))
	if err != nil {
		logger.Log(0, "edr sync: build provider:", err.Error())
		return err
	}
	serialLookup, hasSerialLookup := p.(SerialLookup)
	hostLookup, hasHostLookup := p.(HostEndpointLookup)

	hosts, err := (&schema.Host{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(0, "edr sync: list hosts:", err.Error())
		return err
	}

	var endpoints []ManagedEndpoint
	endpointsLoaded := false

	matched := 0
	for i := range hosts {
		if !hostEligibleForEDR(intg.Provider, hosts[i]) {
			if err := clearHostEDRState(ctx, intg.Provider, hosts[i].ID.String()); err != nil {
				logger.Log(0, "edr sync: clear stale state for host", hosts[i].ID.String(), ":", err.Error())
			}
			continue
		}
		if hasHostLookup {
			ok, err := upsertHostEDRFromHostLookup(ctx, intg.Provider, hostLookup, hosts[i])
			if err != nil {
				logger.Log(0, "edr sync: host lookup for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			if ok {
				matched++
			}
			continue
		}
		if hasSerialLookup && strings.TrimSpace(hosts[i].SerialNumber) != "" {
			ok, err := upsertHostEDRFromSerialLookup(ctx, intg.Provider, serialLookup, hosts[i])
			if err != nil {
				logger.Log(0, "edr sync: serial lookup for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			if ok {
				matched++
			}
			continue
		}
		if !endpointsLoaded {
			endpoints, err = p.ListManagedEndpoints(ctx)
			if err != nil {
				logger.Log(0, "edr sync: list endpoints:", err.Error())
				return err
			}
			endpointsLoaded = true
		}
		found := false
		for _, ep := range endpoints {
			matchedBy, ok := MatchHostToEndpoint(intg.Provider, hosts[i], ep)
			if !ok {
				continue
			}
			if err := upsertHostEDRFromEndpoint(ctx, intg.Provider, hosts[i], ep, matchedBy); err != nil {
				logger.Log(0, "edr sync: upsert state for host", hosts[i].ID.String(), ":", err.Error())
				continue
			}
			matched++
			found = true
			break
		}
		if !found {
			if err := upsertUnmatchedHostEDRState(ctx, intg.Provider, hosts[i]); err != nil {
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
	case ProviderWazuh:
		return h.ID != uuid.Nil
	case ProviderDefender:
		return strings.TrimSpace(h.EntraDeviceID) != "" || strings.TrimSpace(h.SerialNumber) != ""
	default:
		return strings.TrimSpace(h.SerialNumber) != ""
	}
}

func MatchHostToEndpoint(providerID string, h schema.Host, ep ManagedEndpoint) (matchedBy string, ok bool) {
	switch providerID {
	case ProviderWazuh:
		return matchWazuhHostToEndpoint(h, ep)
	case ProviderDefender:
		if entra := strings.TrimSpace(h.EntraDeviceID); entra != "" {
			if deviceEntra := strings.TrimSpace(ep.EntraDeviceID); deviceEntra != "" &&
				strings.EqualFold(normalizeGUID(entra), normalizeGUID(deviceEntra)) {
				return schema.EDRMatchEntraDeviceID, true
			}
		}
		if serialMatch(h.SerialNumber, ep.SerialNumber) {
			return schema.EDRMatchSerialNumber, true
		}
		return "", false
	default:
		if serialMatch(h.SerialNumber, ep.SerialNumber) {
			return schema.EDRMatchSerialNumber, true
		}
		return "", false
	}
}

// SerialMatch reports whether two serial numbers refer to the same device.
func SerialMatch(hostSerial, deviceSerial string) bool {
	return serialMatch(hostSerial, deviceSerial)
}

func serialMatch(hostSerial, deviceSerial string) bool {
	hostSerial = strings.TrimSpace(hostSerial)
	deviceSerial = strings.TrimSpace(deviceSerial)
	return hostSerial != "" && deviceSerial != "" && strings.EqualFold(hostSerial, deviceSerial)
}

func matchWazuhHostToEndpoint(h schema.Host, ep ManagedEndpoint) (matchedBy string, ok bool) {
	if serialMatch(h.SerialNumber, ep.SerialNumber) {
		return schema.EDRMatchSerialNumber, true
	}
	if h.ID != uuid.Nil && strings.EqualFold(h.ID.String(), strings.TrimSpace(ep.Hostname)) {
		return schema.EDRMatchHostID, true
	}
	if labelID := strings.TrimSpace(ep.NetmakerHostID); labelID != "" &&
		h.ID != uuid.Nil && strings.EqualFold(labelID, h.ID.String()) {
		return schema.EDRMatchHostID, true
	}
	if strings.EqualFold(strings.TrimSpace(h.Name), strings.TrimSpace(ep.Hostname)) &&
		HostEndpointIPMatches(h, ep.EndpointIP) {
		return schema.EDRMatchHostname, true
	}
	return "", false
}

// WazuhAgentIPUsable reports whether a Wazuh agent-reported IP is suitable for correlation.
func WazuhAgentIPUsable(ip string) bool {
	ip = strings.TrimSpace(strings.ToLower(ip))
	if ip == "" || ip == "any" {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return !parsed.IsLoopback()
}

// HostEndpointIPMatches reports whether a Wazuh agent IP equals the host WireGuard endpoint.
func HostEndpointIPMatches(h schema.Host, agentIP string) bool {
	if !WazuhAgentIPUsable(agentIP) {
		return false
	}
	parsed := net.ParseIP(strings.TrimSpace(agentIP))
	if parsed == nil {
		return false
	}
	if len(h.EndpointIP) > 0 && h.EndpointIP.Equal(parsed) {
		return true
	}
	if len(h.EndpointIPv6) > 0 && h.EndpointIPv6.Equal(parsed) {
		return true
	}
	return false
}

func normalizeGUID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "{")
	id = strings.TrimSuffix(id, "}")
	return id
}

func upsertUnmatchedHostEDRState(ctx context.Context, providerID string, h schema.Host) error {
	state := schema.DeviceEDRState{
		HostID:         h.ID.String(),
		TenantID:       h.TenantID,
		Provider:       providerID,
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
