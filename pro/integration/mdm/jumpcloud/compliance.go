package jumpcloud

import (
	"strings"
)

const systemPolicyStatusesPath = "/api/v2/systems/"

// deviceTrustCompliant reports whether a system satisfies the configured device-trust
// baseline using JumpCloud policy statuses (GET /api/v2/systems/{id}/policystatuses).
// JumpCloud does not expose a single "device trust" field; policy results are the
// supported API signal for whether bound policies (including trust/security baselines)
// are successfully applied on the device.
func deviceTrustCompliant(results []policyResult, filterPolicyIDs map[string]struct{}) bool {
	if len(results) == 0 {
		// No bound policies with results — nothing failed on the device.
		return true
	}
	checked := 0
	for _, r := range results {
		if len(filterPolicyIDs) > 0 {
			if _, ok := filterPolicyIDs[r.PolicyID]; !ok {
				continue
			}
		}
		checked++
		if policyResultFailed(r) {
			return false
		}
	}
	if len(filterPolicyIDs) > 0 && checked == 0 {
		// Filtered policy IDs are not bound to this system.
		return false
	}
	return true
}

func policyResultFailed(r policyResult) bool {
	if r.Success != nil && !*r.Success {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(r.State)) {
	case "failed", "error":
		return true
	}
	return false
}

func policyIDSet(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

type policyResult struct {
	PolicyID string `json:"policy_id"`
	State    string `json:"state"`
	Success  *bool  `json:"success"`
}
