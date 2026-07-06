//go:build ee
// +build ee

package logic

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// RegisterDeviceHooks wires Pro device API extensions. Called from pro/initialize.go.
func RegisterDeviceHooks() {
	logic.EnrichDeviceNetworksWithJIT = enrichDeviceNetworksWithJIT
}

func enrichDeviceNetworksWithJIT(user *schema.User, accessibleNets []schema.Network, networks []models.DeviceNetwork) []models.DeviceNetwork {
	if len(networks) == 0 {
		return networks
	}
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableJIT {
		return networks
	}

	netIndex := make(map[string]int, len(networks))
	for i, dn := range networks {
		netIndex[dn.NetworkID] = i
	}

	jitStatuses, err := GetUserJITNetworksStatus(accessibleNets, user)
	if err != nil {
		return networks
	}

	for _, js := range jitStatuses {
		idx, ok := netIndex[js.NetworkID]
		if !ok {
			continue
		}
		dn := networks[idx]
		dn.JITEnabled = js.JITEnabled
		dn.JITAppliesToUser = js.JitAppliesToUser
		dn.HasJITAccess = js.HasAccess
		dn.JITPendingRequest = js.PendingRequest
		if js.Grant != nil {
			dn.JITGrant = js.Grant
			exp := js.Grant.ExpiresAt.Unix()
			dn.JITExpiresAt = &exp
		}
		if js.Request != nil {
			dn.JITRequest = js.Request
		}
		if js.JitAppliesToUser && !js.HasAccess && !dn.Joined {
			dn.Status = models.DeviceNetworkStatusJITRequired
			dn.HasJITAccess = false
		}
		networks[idx] = dn
	}
	return networks
}
