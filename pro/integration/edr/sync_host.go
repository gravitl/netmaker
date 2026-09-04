package edr

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gravitl/netmaker/schema"
)

// RefreshHostEDRState syncs EDR posture state for a single host before join or
// registration posture evaluation. It does not honour the global sync rate limit.
func RefreshHostEDRState(ctx context.Context, h schema.Host) error {
	intg, err := GetActive(ctx)
	if err != nil {
		return err
	}
	if intg == nil {
		return nil
	}
	if !hostEligibleForEDR(intg.Provider, h) {
		return clearHostEDRState(ctx, intg.Provider, h.ID.String())
	}
	p, err := Build(intg.Provider, json.RawMessage(intg.Config))
	if err != nil {
		return err
	}
	if lookup, ok := p.(HostEndpointLookup); ok {
		_, err := upsertHostEDRFromHostLookup(ctx, intg.Provider, lookup, h)
		return err
	}
	if lookup, ok := p.(SerialLookup); ok && strings.TrimSpace(h.SerialNumber) != "" {
		_, err := upsertHostEDRFromSerialLookup(ctx, intg.Provider, lookup, h)
		return err
	}
	return refreshHostEDRByListing(ctx, intg.Provider, p, h)
}
