package migrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
)

// backfillNetworkGlobalAccess sets network_global_access on default network-admin
// roles that predate the column (NM-378 removed TenantGlobalAccess on network
// roles; NM-356 added NetworkGlobalAccess as the replacement). Upsert of default
// role templates also sets the flag; this UPDATE covers rows that were never
// rewritten and any tenant where init silently failed.
func backfillNetworkGlobalAccess(ctx context.Context) {
	roles, err := (&schema.UserRole{}).ListNetworkRoles(ctx)
	if err != nil {
		logger.Log(0, "migration: failed to list network roles for network_global_access backfill:", err.Error())
		return
	}

	updated := 0
	for _, role := range roles {
		if role.NetworkGlobalAccess || !isDefaultNetworkAdminRoleSlug(role) {
			continue
		}
		res := db.FromContext(ctx).Model(&schema.UserRole{}).
			Where("id = ?", role.ID).
			Update("network_global_access", true)
		if res.Error != nil {
			logger.Log(0, "migration: failed to backfill network_global_access for role", role.Slug.String(), res.Error.Error())
			continue
		}
		if res.RowsAffected > 0 {
			updated++
		}
	}
	if updated > 0 {
		logger.Log(1, fmt.Sprintf("migration: set network_global_access=true on %d network-admin role(s)", updated))
	}
}

func isDefaultNetworkAdminRoleSlug(role schema.UserRole) bool {
	if !role.Default || role.NetworkID == "" {
		return false
	}
	return strings.HasSuffix(role.Slug.String(), "-"+schema.NetworkAdmin.String())
}
