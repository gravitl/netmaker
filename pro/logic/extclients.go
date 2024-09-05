package logic

import (
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// GetExtClientExpiryTime - returns the expiry time for the external client
func GetExtClientExpiryTime(owner *models.User) time.Time {
	if servercfg.GetServerConfig().RacAutoDisable {
		if owner.PlatformRoleID == models.SuperAdminRole || owner.PlatformRoleID == models.AdminRole {
			return time.Time{}
		}
		validityDuration := servercfg.GetJwtValidityDuration()
		return owner.LastLoginTime.Add(validityDuration)
	}
	return time.Time{}
}
