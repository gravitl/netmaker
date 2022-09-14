package pro

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
)

// AddProNetDefaults - adds default values to a network model
func AddProNetDefaults(network *models.Network) {
	if network.ProSettings == nil {
		newProSettings := promodels.ProNetwork{
			DefaultAccessLevel:     NO_ACCESS,
			DefaultUserNodeLimit:   0,
			DefaultUserClientLimit: 0,
			AllowedUsers:           []string{},
			AllowedGroups:          []string{DEFAULT_ALLOWED_GROUPS},
		}
		network.ProSettings = &newProSettings
	}
	if network.ProSettings.AllowedUsers == nil {
		network.ProSettings.AllowedUsers = []string{}
	}
	if network.ProSettings.AllowedGroups == nil {
		network.ProSettings.AllowedGroups = []string{DEFAULT_ALLOWED_GROUPS}
	}
}

// isUserGroupAllowed - checks if a user group is allowed on a network
func isUserGroupAllowed(network *models.Network, groupName string) bool {
	if network.ProSettings != nil {
		if len(network.ProSettings.AllowedGroups) > 0 {
			for i := range network.ProSettings.AllowedGroups {
				currentGroup := network.ProSettings.AllowedGroups[i]
				if currentGroup == DEFAULT_ALLOWED_GROUPS || currentGroup == groupName {
					return true
				}
			}
		}
	}
	return false
}

func isUserInAllowedUsers(network *models.Network, userName string) bool {
	if network.ProSettings != nil {
		if len(network.ProSettings.AllowedUsers) > 0 {
			for i := range network.ProSettings.AllowedUsers {
				currentUser := network.ProSettings.AllowedUsers[i]
				if currentUser == DEFAULT_ALLOWED_USERS || currentUser == userName {
					return true
				}
			}
		}
	}
	return false
}

// IsUserAllowed - checks if given username + groups if a user is allowed on network
func IsUserAllowed(network *models.Network, userName string, groups []string) bool {
	isGroupAllowed := false
	for _, g := range groups {
		if isUserGroupAllowed(network, g) {
			isGroupAllowed = true
			break
		}
	}

	return isUserInAllowedUsers(network, userName) || isGroupAllowed
}
