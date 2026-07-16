package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

// Run - runs all migrations
func Run() {
	tenants, _ := (&schema.Tenant{}).List(db.WithContext(context.TODO()))
	for _, tenant := range tenants {
		ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, tenant.ID)
		migrateSettings(ctx)
		assignSuperAdmin(ctx)
		logic.IntialiseGroups(ctx)

		networks, _ := (&schema.Network{}).ListAll(ctx)
		for _, netI := range networks {
			logic.CreateDefaultNetworkRolesAndGroups(ctx, schema.NetworkID(netI.Name))
		}

		createDefaultTagsAndPolicies(ctx)
		cleanupDeletedUserGroupRefs(ctx)
		updateNewAcls(ctx)
		updateNodes(ctx)
		logic.CleanupGwsMigration(ctx)
		deleteOldExtclients(ctx)
	}

	updateEnrollmentKeys()
	syncUsers()
	migrateEgressDomains()
	updateNetworks()
	migrateNameservers()
	migrateEgressNatMode()
	cleanUpDeleteNetworksRefs()

	logic.InitialiseRoles()
}

func updateNetworks() {
	initializeVirtualNATSettings()
}

func initializeVirtualNATSettings() {
	if !servercfg.IsPro {
		return
	}
	logger.Log(1, "Initializing Virtual NAT settings for existing networks")
	defer logger.Log(1, "Completed initializing Virtual NAT settings for existing networks")

	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		logger.Log(0, "failed to get networks for Virtual NAT migration:", err.Error())
		return
	}

	allocatedPools := make(map[string]struct{})

	for _, network := range networks {
		if isValidVNATPool(network.VirtualNATPoolIPv4) && network.VirtualNATSitePrefixLenIPv4 > 0 {
			allocatedPools[network.VirtualNATPoolIPv4] = struct{}{}
		}
	}

	_, fallbackNet, err := net.ParseCIDR(logic.FallbackVNATPool)
	if err != nil || fallbackNet == nil {
		logger.Log(0, "failed to parse fallback pool for Virtual NAT migration:", err.Error())
		return
	}
	_, cgnatNet, err := net.ParseCIDR(logic.CgnatCIDR)
	if err != nil || cgnatNet == nil {
		logger.Log(0, "failed to parse CGNAT CIDR for Virtual NAT migration:", err.Error())
		return
	}

	for _, network := range networks {
		if isValidVNATPool(network.VirtualNATPoolIPv4) && network.VirtualNATSitePrefixLenIPv4 > 0 {
			continue
		}

		vpnCIDR := network.AddressRange
		needsUniquePool := false

		if vpnCIDR == "" {
			needsUniquePool = true
		} else {
			_, vpnNet, err := net.ParseCIDR(vpnCIDR)
			if err != nil || vpnNet == nil {
				needsUniquePool = true
			} else if vpnNet.Contains(cgnatNet.IP) || cgnatNet.Contains(vpnNet.IP) {
				needsUniquePool = true
			}
		}

		if needsUniquePool {
			uniquePool := logic.AllocateUniquePoolFromFallback(fallbackNet, logic.VNATPoolPrefixLen, allocatedPools, network.Name)
			if uniquePool == "" {
				logger.Log(0, "failed to allocate unique Virtual NAT pool for network", network.Name, "- pool exhausted")
				continue
			}
			network.VirtualNATPoolIPv4 = uniquePool
			network.VirtualNATSitePrefixLenIPv4 = logic.DefaultSitePrefixV4
			allocatedPools[uniquePool] = struct{}{}
		} else {
			logic.AssignVirtualNATDefaults(&network, vpnCIDR)
		}

		if network.VirtualNATPoolIPv4 == "" {
			logger.Log(0, "skipping Virtual NAT update for network", network.Name, "- no pool assigned")
			continue
		}

		if err := logic.UpsertNetwork(&network); err != nil {
			logger.Log(0, "failed to update network", network.Name, "with Virtual NAT settings:", err.Error())
			continue
		}
		logger.Log(1, "initialized Virtual NAT settings for network", network.Name, "pool:", network.VirtualNATPoolIPv4)
	}
}

func isValidVNATPool(pool string) bool {
	if pool == "" {
		return false
	}
	_, _, err := net.ParseCIDR(pool)
	return err == nil
}

func assignSuperAdmin(ctx context.Context) {
	exists, _ := (&schema.User{}).SuperAdminExists(ctx)
	if exists {
		return
	}

	defaultTenant := &schema.Tenant{}
	err := defaultTenant.GetDefault(db.WithContext(context.TODO()))
	if err != nil {
		return
	}

	users, err := (&schema.User{}).ListAll(ctx)
	if err != nil || len(users) == 0 {
		return
	}

	createdSuperAdmin := false
	owner := servercfg.GetOwnerEmail()
	if owner != "" {
		user := &schema.User{Username: owner}
		err = user.Get(ctx)
		if err != nil {
			log.Fatal("error getting user", "user", owner, "error", err.Error())
		}

		tm := &schema.TenantMembership{
			TenantID: scope.ID(ctx),
			UserID:   user.ID,
			RoleID:   schema.SuperAdminRole,
		}
		err = tm.UpdateRoleID(ctx)
		if err != nil {
			log.Fatal(
				"error updating user to superadmin",
				"user",
				user.Username,
				"error",
				err.Error(),
			)
		}
		return
	}

	for _, user := range users {
		if user.PlatformRoleID != schema.AdminRole {
			continue
		}

		tm := &schema.TenantMembership{
			TenantID: scope.ID(ctx),
			UserID:   user.ID,
			RoleID:   schema.SuperAdminRole,
		}
		err = tm.UpdateRoleID(ctx)
		if err != nil {
			slog.Error(
				"error updating user to superadmin",
				"user",
				user.Username,
				"error",
				err.Error(),
			)
			continue
		}
		createdSuperAdmin = true
		break
	}

	if !createdSuperAdmin {
		slog.Error("failed to create superadmin!!")
	}
}

func updateEnrollmentKeys() {
	ctx := db.WithContext(context.TODO())
	existingKeys, err := logic.GetAllEnrollmentKeys(ctx)
	if err != nil {
		return
	}
	// check if any networks already have a default enrollment key
	existingNetworks := make(map[string]struct{})
	for _, existingKey := range existingKeys {
		if existingKey.Default {
			for _, n := range existingKey.Networks {
				existingNetworks[n] = struct{}{}
			}
		}
	}
	networks, _ := (&schema.Network{}).ListAll(ctx)
	for _, network := range networks {
		if _, ok := existingNetworks[network.Name]; ok {
			continue
		}

		ctx := scope.WithContext(ctx, scope.TenantScope, network.TenantID)
		_, _ = logic.CreateDefaultNetworkEnrollmentKey(ctx, network.Name)
	}
}

func updateNodes(ctx context.Context) {
	nodes, err := logic.GetAllNodes(ctx)
	if err != nil {
		slog.Error("migration failed for nodes", "error", err)
		return
	}
	for _, node := range nodes {
		node := node
		if node.IsIngressGateway {
			host := &schema.Host{
				ID: node.HostID,
			}
			err = host.Get(ctx)
			if err == nil {
				go logic.DeleteRole(models.GetRAGRoleID(node.Network, host.ID.String()), true)
			}
		}
	}
	extclients, _ := logic.GetAllExtClients(ctx)
	for _, extclient := range extclients {
		if extclient.Tags == nil {
			extclient.Tags = make(map[models.TagID]struct{})
			logic.SaveExtClient(ctx, &extclient)
		}
	}
}

func removeInterGw(egressRanges []string) ([]string, bool) {
	update := false
	for i := len(egressRanges) - 1; i >= 0; i-- {
		if egressRanges[i] == "0.0.0.0/0" || egressRanges[i] == "::/0" {
			update = true
			egressRanges = append(egressRanges[:i], egressRanges[i+1:]...)
		}
	}
	return egressRanges, update
}

func updateNewAcls(ctx context.Context) {
	if servercfg.IsPro {
		userGroups, _ := (&schema.UserGroup{}).ListAll(ctx)
		for _, userGroup := range userGroups {
			group := userGroup
			if group.Default {
				continue
			}

			networks, err := logic.GetGroupNetworksMap(&group)
			if err != nil {
				continue
			}

			for networkID, network := range networks {
				createSeparateACL := false
				enableSeparateACL := true
				adminAcl, err := logic.GetAcl(ctx, fmt.Sprintf("%s.%s-grp", networkID, schema.NetworkAdmin))
				if err == nil {
					var newAclSrc []models.AclPolicyTag
					for _, src := range adminAcl.Src {
						if src.ID == models.UserGroupAclID && src.Value == group.ID.String() {
							createSeparateACL = true
							enableSeparateACL = adminAcl.Enabled
						} else {
							newAclSrc = append(newAclSrc, src)
						}
					}

					adminAcl.Src = newAclSrc
					_ = logic.UpsertAcl(ctx, adminAcl)
				}

				userAcl, err := logic.GetAcl(ctx, fmt.Sprintf("%s.%s-grp", networkID, schema.NetworkUser))
				if err == nil {
					var newAclSrc []models.AclPolicyTag
					for _, src := range userAcl.Src {
						if src.ID == models.UserGroupAclID && src.Value == group.ID.String() {
							if !createSeparateACL {
								// if group src not found in adminACL, then create.
								createSeparateACL = true
								enableSeparateACL = userAcl.Enabled
							} else {
								// if group src found in adminACL, then create.
								// additionally, if either policy allows access, then allow access.
								if !enableSeparateACL {
									enableSeparateACL = adminAcl.Enabled
								}
							}
						} else {
							newAclSrc = append(newAclSrc, src)
						}
					}

					userAcl.Src = newAclSrc
					_ = logic.UpsertAcl(ctx, userAcl)
				}

				expectedAcl := models.Acl{
					ID:          uuid.New().String(),
					Name:        fmt.Sprintf("%s group", group.Name),
					MetaData:    "This Policy allows user group to communicate with all gateways",
					Default:     true,
					ServiceType: models.Any,
					NetworkID:   schema.NetworkID(network.Name),
					Proto:       models.ALL,
					RuleType:    models.UserPolicy,
					Src: []models.AclPolicyTag{
						{
							ID:    models.UserGroupAclID,
							Value: group.ID.String(),
						},
					},
					Dst: []models.AclPolicyTag{
						{
							ID:    models.NodeTagID,
							Value: fmt.Sprintf("%s.%s", schema.NetworkID(network.Name), models.GwTagName),
						}},
					AllowedDirection: models.TrafficDirectionUni,
					Enabled:          true,
					CreatedBy:        "auto",
					CreatedAt:        time.Now().UTC(),
				}

				acls, _ := logic.ListAclsByNetwork(ctx, networkID)
				for _, acl := range acls {
					if acl.Name == expectedAcl.Name &&
						acl.MetaData == expectedAcl.MetaData &&
						acl.ServiceType == expectedAcl.ServiceType &&
						acl.NetworkID == expectedAcl.NetworkID &&
						acl.Proto == expectedAcl.Proto &&
						acl.RuleType == expectedAcl.RuleType &&
						slices.Equal(acl.Src, expectedAcl.Src) &&
						slices.Equal(acl.Dst, expectedAcl.Dst) &&
						acl.AllowedDirection == expectedAcl.AllowedDirection {

						acl.Default = true
						_ = logic.UpsertAcl(ctx, acl)
						createSeparateACL = false
						break
					}
				}

				if createSeparateACL {
					expectedAcl.Enabled = enableSeparateACL
					_ = logic.InsertAcl(ctx, expectedAcl)
				}
			}

			_ = logic.EnsureDefaultUserGroupNetworkPolicies(ctx, nil, &group)
		}
	}
}

func MigrateEmqx() {

	err := mq.SendPullSYN()
	if err != nil {
		logger.Log(0, "failed to send pull syn to clients", "error", err.Error())

	}
	time.Sleep(time.Second * 3)
	slog.Info("proceeding to kicking out clients from emqx")
	err = mq.KickOutClients()
	if err != nil {
		logger.Log(2, "failed to migrate emqx: ", "kickout-error", err.Error())
	}

}

func syncUsers() {
	logger.Log(1, "Migrating Users (SyncUsers)")
	defer logger.Log(1, "Completed migrating Users (SyncUsers)")

	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err == nil {
		for _, user := range users {
			user := user
			user.AuthType = schema.BasicAuth
			if logic.IsOauthUser(&user) == nil {
				user.AuthType = schema.OAuth
			}

			// Do not call AddGlobalNetRolesToAdmins here: this runs on every server
			// start and would re-assign the global admin group to elevated users who
			// intentionally cleared groups. On role upgrade to admin/super-admin with no
			// groups, UpdateUser assigns the global admin group via AddGlobalGroupOnRoleUpgrade.
			logic.UpsertUser(user)
		}
	}

}

func createDefaultTagsAndPolicies(ctx context.Context) {
	networks, err := (&schema.Network{}).ListAll(ctx)
	if err != nil {
		return
	}

	for _, network := range networks {
		logic.CreateDefaultTags(ctx, schema.NetworkID(network.Name))
		logic.CreateDefaultAclNetworkPolicies(ctx, schema.NetworkID(network.Name))
		// delete old remote access gws policy
		_ = logic.DeleteAcl(ctx, models.Acl{ID: fmt.Sprintf("%s.%s", network.Name, "all-remote-access-gws")})
	}

	logic.MigrateAclPolicies(ctx)
	if !servercfg.IsPro {
		nodes, _ := logic.GetAllNodes(ctx)
		for _, node := range nodes {
			if node.IsGw {
				node.Tags = make(map[models.TagID]struct{})
				node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
				logic.UpsertNode(&node)
			}
		}
	}
}

// migrateEgressDomains copies the legacy DB column "domain" (singular) into "domains" only.
// Resolved IPs use domain_ans_by_domain at runtime; legacy domain_ans is not migrated.
func migrateEgressDomains() {
	egs, err := (&schema.Egress{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		logger.Log(0, "migration: failed to list egresses:", err.Error())
		return
	}
	gormDB := db.FromContext(db.WithContext(context.TODO()))
	for _, eg := range egs {
		if len(eg.Domains) > 0 {
			continue
		}
		var legacyDomainPtr *string
		if err := gormDB.Table(eg.Table()).Where("id = ?", eg.ID).Select("domain").Scan(&legacyDomainPtr).Error; err != nil {
			logger.Log(0, "migration: failed to read legacy egress domain:", eg.ID, err.Error())
			continue
		}
		if legacyDomainPtr == nil {
			continue
		}
		legacyDomain := strings.TrimSpace(*legacyDomainPtr)
		if legacyDomain == "" {
			continue
		}
		normDomains, err := logic.NormalizeEgressReqDomains([]string{legacyDomain})
		if err != nil || len(normDomains) == 0 {
			logger.Log(0, "migration: invalid legacy egress domain:", eg.ID, legacyDomain)
			continue
		}
		if err := gormDB.Table(eg.Table()).Where("id = ?", eg.ID).Updates(map[string]any{
			"domains": datatypes.JSONSlice[string](normDomains),
		}).Error; err != nil {
			logger.Log(0, "migration: failed to update egress domains:", eg.ID, err.Error())
		}
	}
}

func migrateSettings(ctx context.Context) {
	settingsRecord := &schema.TenantSettingsRecord{Key: scope.ID(ctx)}
	err := settingsRecord.Get(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = logic.UpsertServerSettings(ctx, logic.GetServerSettingsFromEnv())
	}
	settings := logic.GetServerSettings(ctx)
	if settings.PeerConnectionCheckInterval == "" {
		settings.PeerConnectionCheckInterval = "15"
	}
	if settings.PostureCheckInterval == "" {
		settings.PostureCheckInterval = "30"
	}
	if settings.CleanUpInterval == 0 {
		settings.CleanUpInterval = 10
	}
	if settings.IPDetectionInterval == 0 {
		settings.IPDetectionInterval = 15
	}
	if settings.AuditLogsRetentionPeriodInDays == 0 {
		settings.AuditLogsRetentionPeriodInDays = 7
	}
	if settings.DefaultDomain == "" {
		settings.DefaultDomain = servercfg.GetDefaultDomain()
	}
	if settings.JwtValidityDurationClients == 0 {
		settings.JwtValidityDurationClients = servercfg.GetJwtValidityDurationFromEnv() / 60
	}
	if settings.StunServers == "" {
		settings.StunServers = servercfg.GetStunServers()
	}
	if settings.SmtpHost != "" {
		var rawValue []byte
		_ = db.FromContext(ctx).Table(settingsRecord.TableName()).
			Where("key = ?", scope.ID(ctx)).Pluck("value", &rawValue).Error
		var settingsD map[string]any
		_ = json.Unmarshal(rawValue, &settingsD)
		if _, ok := settingsD["smtp_skip_tls_verify"]; !ok {
			// skip tls verification for older deployments when tls verification wasn't configurable.
			settings.SmtpSkipTlsVerify = true
		}
	}
	_ = logic.UpsertServerSettings(ctx, settings)
}

func deleteOldExtclients(ctx context.Context) {
	extclients, _ := logic.GetAllExtClients(ctx)
	userExtclientMap := make(map[string][]models.ExtClient)
	for _, extclient := range extclients {
		if extclient.RemoteAccessClientID == "" {
			continue
		}

		if extclient.Enabled {
			continue
		}

		if _, ok := userExtclientMap[extclient.OwnerID]; !ok {
			userExtclientMap[extclient.OwnerID] = make([]models.ExtClient, 0)
		}

		userExtclientMap[extclient.OwnerID] = append(userExtclientMap[extclient.OwnerID], extclient)
	}

	for _, userExtclients := range userExtclientMap {
		if len(userExtclients) > 1 {
			for _, extclient := range userExtclients[1:] {
				_ = logic.DeleteExtClient(ctx, extclient.Network, extclient.Network, false)
			}
		}
	}
}

func cleanupDeletedUserGroupRefs(ctx context.Context) {
	groups, err := (&schema.UserGroup{}).ListAll(ctx)
	if err != nil {
		// skip if we can't list all groups.
		return
	}

	existingGroups := make(map[schema.UserGroupID]schema.UserGroup)
	for _, group := range groups {
		existingGroups[group.ID] = group
	}

	existingUsers := make(map[string]schema.User)
	users, _ := (&schema.User{}).ListAll(ctx)
	for _, user := range users {
		existingUsers[user.Username] = user
		var update bool
		for groupID := range user.UserGroups.Data() {
			if _, ok := existingGroups[groupID]; !ok {
				delete(user.UserGroups.Data(), groupID)
				update = true
			}
		}

		if update {
			_ = user.Update(ctx)
		}
	}

	for _, acl := range logic.ListAcls(ctx) {
		var newSrc []models.AclPolicyTag
		for _, src := range acl.Src {
			if src.ID == models.UserGroupAclID {
				if group, ok := existingGroups[schema.UserGroupID(src.Value)]; ok {
					var hasAccess bool
					if _, ok := group.NetworkRoles.Data()[schema.AllNetworks]; ok {
						hasAccess = true
					}

					if _, ok := group.NetworkRoles.Data()[acl.NetworkID]; ok {
						hasAccess = true
					}

					if hasAccess {
						newSrc = append(newSrc, src)
					}
				}
			} else if src.ID == models.UserAclID && src.Value != "*" {
				if _, ok := existingUsers[src.Value]; ok {
					newSrc = append(newSrc, src)
				}
			} else {
				newSrc = append(newSrc, src)
			}
		}

		if len(newSrc) == 0 {
			_ = logic.DeleteAcl(ctx, acl)
		} else if len(acl.Src) != len(newSrc) {
			acl.Src = newSrc
			_ = logic.UpsertAcl(ctx, acl)
		}
	}

	postureChecks, _ := (&schema.PostureCheck{}).ListAll(ctx)
	for _, postureCheck := range postureChecks {
		var update bool
		for groupID := range postureCheck.UserGroups {
			if groupID != "*" {
				if _, ok := existingGroups[schema.UserGroupID(groupID)]; !ok {
					delete(postureCheck.UserGroups, groupID)
					update = true
				}
			}
		}

		if update {
			_ = postureCheck.Update(ctx)
		}
	}
}

func migrateNameservers() {
	networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	for _, network := range networks {
		_ = logic.CreateFallbackNameserver(&network)
	}

	nameservers, _ := (&schema.Nameserver{}).ListAll(db.WithContext(context.TODO()))
	for _, nameserver := range nameservers {
		if len(nameserver.Domains) != 0 {
			for _, matchDomain := range nameserver.MatchDomains {
				nameserver.Domains = append(nameserver.Domains, schema.NameserverDomain{
					Domain: matchDomain,
				})
			}

			nameserver.MatchDomains = []string{}

			_ = nameserver.Update(db.WithContext(context.TODO()))
		}
	}
}

func migrateEgressNatMode() {
	egresses, _ := (&schema.Egress{}).ListAll(db.WithContext(context.TODO()))
	for _, egress := range egresses {
		if egress.Nat {
			if egress.Mode == "" {
				egress.Mode = schema.DirectNAT
			}
		} else {
			egress.Mode = schema.DisabledNAT
		}

		_ = egress.Update(db.WithContext(context.TODO()))
	}
}

func cleanUpDeleteNetworksRefs() {
	networksMap := make(map[string]bool)
	networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	for _, network := range networks {
		networksMap[network.Name] = true
	}

	dnsRecords, _ := (&schema.DNSRecord{}).List(db.WithContext(context.TODO()))
	for _, r := range dnsRecords {
		_, ok := networksMap[r.Value.Data().Network]
		if !ok {
			_ = (&schema.DNSRecord{Key: r.Key}).Delete(db.WithContext(context.TODO()))
		}
	}

	nameservers, _ := (&schema.Nameserver{}).ListAll(db.WithContext(context.TODO()))
	for _, nameserver := range nameservers {
		_, ok := networksMap[nameserver.NetworkID]
		if !ok {
			_ = nameserver.Delete(db.WithContext(context.TODO()))
		}
	}

	egresses, _ := (&schema.Egress{}).ListAll(db.WithContext(context.TODO()))
	for _, egress := range egresses {
		_, ok := networksMap[egress.Network]
		if !ok {
			_ = egress.Delete(db.WithContext(context.TODO()))
		}
	}

	postureChecks, _ := (&schema.PostureCheck{}).ListAll(db.WithContext(context.TODO()))
	for _, postureCheck := range postureChecks {
		_, ok := networksMap[postureCheck.NetworkID.String()]
		if !ok {
			_ = postureCheck.Delete(db.WithContext(context.TODO()))
		}
	}
}
