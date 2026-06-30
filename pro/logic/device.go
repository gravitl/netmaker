//go:build ee
// +build ee

package logic

import (
	"errors"
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// RegisterDeviceHooks wires Pro device API extensions. Called from pro/initialize.go.
func RegisterDeviceHooks() {
	logic.EnrichDeviceNetworksWithJIT = enrichDeviceNetworksWithJIT
	logic.RequestDeviceJITAccess = requestDeviceJITAccess
}

func enrichDeviceNetworksWithJIT(user *schema.User, networks []models.DeviceNetwork) []models.DeviceNetwork {
	if len(networks) == 0 {
		return networks
	}
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableJIT {
		return networks
	}

	accessibleNets := make([]schema.Network, 0, len(networks))
	netIndex := make(map[string]int, len(networks))
	for i, dn := range networks {
		accessibleNets = append(accessibleNets, schema.Network{Name: dn.NetworkID})
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

func requestDeviceJITAccess(user *schema.User, networkID, reason string) (any, error) {
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableJIT {
		return nil, errors.New("JIT feature is not enabled")
	}
	if !logic.IsUserAllowedToJoinNetwork(user.Username, networkID) {
		return nil, errors.New("user does not have access to network")
	}
	req, err := CreateJITRequest(networkID, user.Username, reason)
	if err != nil {
		return nil, err
	}
	_ = time.Now()
	return req, nil
}
