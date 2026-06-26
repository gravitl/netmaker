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
	if !hostEligibleForEDR(intg.ID, h) {
		return clearHostEDRState(ctx, intg.ID, h.ID.String())
	}
	p, err := Build(intg.ID, json.RawMessage(intg.Config))
	if err != nil {
		return err
	}
	if lookup, ok := p.(HostEndpointLookup); ok {
		_, err := upsertHostEDRFromHostLookup(ctx, intg.ID, lookup, h)
		return err
	}
	if lookup, ok := p.(SerialLookup); ok && strings.TrimSpace(h.SerialNumber) != "" {
		_, err := upsertHostEDRFromSerialLookup(ctx, intg.ID, lookup, h)
		return err
	}
	return refreshHostEDRByListing(ctx, intg.ID, p, h)
}
