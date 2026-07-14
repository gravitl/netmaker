package edr

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// SerialLookup is implemented by EDR providers that can resolve a host by
// serial_number via a targeted API query instead of listing all endpoints.
type SerialLookup interface {
	LookupBySerial(ctx context.Context, serial string) (ManagedEndpoint, error)
}

// HostEndpointLookup resolves a host using provider-specific filtered queries
// (e.g. serial_number) without listing the full fleet.
type HostEndpointLookup interface {
	LookupForHost(ctx context.Context, h schema.Host) (ManagedEndpoint, string, error)
}

func upsertHostEDRFromHostLookup(
	ctx context.Context,
	providerID string,
	lookup HostEndpointLookup,
	h schema.Host,
) (bool, error) {
	ep, matchedBy, err := lookup.LookupForHost(ctx, h)
	state := schema.DeviceEDRState{
		HostID:       h.ID.String(),
		Provider:     providerID,
		MatchedBy:    matchedBy,
		LastSyncedAt: time.Now().UTC(),
	}
	if code := LookupErrorCode(err); code != "" {
		state.LastError = code
		state.AgentInstalled = false
		state.AgentHealthy = false
		state.RiskLevel = string(RiskCritical)
		return false, state.Upsert(db.WithContext(ctx))
	}
	if err != nil {
		return false, err
	}
	raw := ep.RawVendorData
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	state.EDRDeviceID = ep.ProviderDeviceID
	state.AgentInstalled = ep.AgentInstalled
	state.AgentHealthy = ep.AgentHealthy
	state.RiskLevel = string(ep.RiskLevel)
	state.ThreatCount = ep.ThreatCount
	state.ActiveThreats = ep.ActiveThreats
	state.Isolated = ep.Isolated
	state.Contained = ep.Contained
	state.LastSeenAt = ep.LastSeen
	state.LastError = ""
	state.RawVendorData = datatypes.JSON(raw)
	return true, state.Upsert(db.WithContext(ctx))
}

func upsertHostEDRFromSerialLookup(
	ctx context.Context,
	providerID string,
	lookup SerialLookup,
	h schema.Host,
) (bool, error) {
	ep, err := lookup.LookupBySerial(ctx, h.SerialNumber)
	state := schema.DeviceEDRState{
		HostID:       h.ID.String(),
		Provider:     providerID,
		MatchedBy:    schema.EDRMatchSerialNumber,
		LastSyncedAt: time.Now().UTC(),
	}
	if code := LookupErrorCode(err); code != "" {
		state.LastError = code
		state.AgentInstalled = false
		state.AgentHealthy = false
		state.RiskLevel = string(RiskCritical)
		return false, state.Upsert(db.WithContext(ctx))
	}
	if err != nil {
		return false, err
	}
	raw := ep.RawVendorData
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	state.EDRDeviceID = ep.ProviderDeviceID
	state.AgentInstalled = ep.AgentInstalled
	state.AgentHealthy = ep.AgentHealthy
	state.RiskLevel = string(ep.RiskLevel)
	state.ThreatCount = ep.ThreatCount
	state.ActiveThreats = ep.ActiveThreats
	state.Isolated = ep.Isolated
	state.Contained = ep.Contained
	state.LastSeenAt = ep.LastSeen
	state.LastError = ""
	state.RawVendorData = datatypes.JSON(raw)
	return true, state.Upsert(db.WithContext(ctx))
}

func upsertHostEDRFromEndpoint(
	ctx context.Context,
	providerID string,
	h schema.Host,
	ep ManagedEndpoint,
	matchedBy string,
) error {
	raw := ep.RawVendorData
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	state := schema.DeviceEDRState{
		HostID:         h.ID.String(),
		Provider:       providerID,
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
	return state.Upsert(db.WithContext(ctx))
}

func refreshHostEDRByListing(
	ctx context.Context,
	providerID string,
	p Provider,
	h schema.Host,
) error {
	endpoints, err := p.ListManagedEndpoints(ctx)
	if err != nil {
		return err
	}
	for _, ep := range endpoints {
		matchedBy, ok := MatchHostToEndpoint(providerID, h, ep)
		if !ok {
			continue
		}
		return upsertHostEDRFromEndpoint(ctx, providerID, h, ep, matchedBy)
	}
	return upsertUnmatchedHostEDRState(ctx, providerID, h.ID.String())
}
