package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/exp/slog"
	"gorm.io/datatypes"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

// Run - runs all migrations
func Run() {
	migrateSettings()
	updateEnrollmentKeys()
	assignSuperAdmin()
	createDefaultTagsAndPolicies()
	syncUsers()
	updateNodes()
	updateAcls()
	updateNewAcls()
	logic.MigrateToGws()
	migrateToEgressV1()
	updateNetworks()
	resync()
	deleteOldExtclients()
	checkAndDeprecateOldAcls()
	cleanupDeletedUserGroupRefs()
}

func checkAndDeprecateOldAcls() {
	// check if everything is allowed on old acl and disable old acls
	nets, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	disableOldAcls := true
	for _, netI := range nets {
		networkACL, err := nodeacls.FetchAllACLs(nodeacls.NetworkID(netI.Name))
		if err != nil {
			continue
		}
		for _, aclNode := range networkACL {
			for _, allowed := range aclNode {
				if allowed != acls.Allowed {
					disableOldAcls = false
					break
				}
			}
		}
		if disableOldAcls {
			netI.DefaultACL = "yes"
			logic.UpsertNetwork(&netI)
		}
	}
	if disableOldAcls {
		settings := logic.GetServerSettings()
		settings.OldAClsSupport = false
		logic.UpsertServerSettings(settings)
	}

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

// removes if any stale configurations from previous run.
func resync() {

	nodes, _ := logic.GetAllNodes()
	for _, node := range nodes {
		if !node.IsGw {
			if len(node.RelayedNodes) > 0 {
				logic.DeleteRelay(node.Network, node.ID.String())
			}
			if node.IsIngressGateway {
				logic.DeleteIngressGateway(node.ID.String())
			}
			if len(node.InetNodeReq.InetNodeClientIDs) > 0 || node.IsInternetGateway {
				logic.UnsetInternetGw(&node)
				logic.UpsertNode(&node)
			}
		}
		if node.IsRelayed {
			if node.RelayedBy == "" {
				node.IsRelayed = false
				node.InternetGwID = ""
				logic.UpsertNode(&node)
			}
			if node.RelayedBy != "" {
				// check if node exists
				_, err := logic.GetNodeByID(node.RelayedBy)
				if err != nil {
					node.RelayedBy = ""
					node.InternetGwID = ""
					logic.UpsertNode(&node)
				}
			}
		}
		if node.InternetGwID != "" {
			_, err := logic.GetNodeByID(node.InternetGwID)
			if err != nil {
				node.InternetGwID = ""
				logic.UpsertNode(&node)
			}
		}
	}
}

func assignSuperAdmin() {
	users, err := logic.GetUsers()
	if err != nil || len(users) == 0 {
		return
	}

	if ok, _ := logic.HasSuperAdmin(); ok {
		return
	}
	createdSuperAdmin := false
	owner := servercfg.GetOwnerEmail()
	if owner != "" {
		user := &schema.User{Username: owner}
		err = user.Get(db.WithContext(context.TODO()))
		if err != nil {
			log.Fatal("error getting user", "user", owner, "error", err.Error())
		}
		user.PlatformRoleID = schema.SuperAdminRole
		err = logic.UpsertUser(*user)
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
	for _, u := range users {
		var isAdmin bool
		if u.PlatformRoleID == schema.AdminRole {
			isAdmin = true
		}
		if u.PlatformRoleID == "" && u.IsAdmin {
			isAdmin = true
		}

		if isAdmin {
			user := &schema.User{Username: u.UserName}
			err = user.Get(db.WithContext(context.TODO()))
			if err != nil {
				slog.Error("error getting user", "user", u.UserName, "error", err.Error())
				continue
			}
			user.PlatformRoleID = schema.SuperAdminRole
			err = logic.UpsertUser(*user)
			if err != nil {
				slog.Error(
					"error updating user to superadmin",
					"user",
					user.Username,
					"error",
					err.Error(),
				)
				continue
			} else {
				createdSuperAdmin = true
			}
			break
		}
	}

	if !createdSuperAdmin {
		slog.Error("failed to create superadmin!!")
	}
}

func updateEnrollmentKeys() {
	rows, err := database.FetchRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
	if err != nil {
		return
	}
	for _, row := range rows {
		var key models.EnrollmentKey
		if err = json.Unmarshal([]byte(row), &key); err != nil {
			continue
		}
		if key.Type != models.Undefined {
			logger.Log(2, "migration: enrollment key type already set")
			continue
		} else {
			logger.Log(2, "migration: updating enrollment key type")
			if key.Unlimited {
				key.Type = models.Unlimited
			} else if key.UsesRemaining > 0 {
				key.Type = models.Uses
			} else if !key.Expiration.IsZero() {
				key.Type = models.TimeExpiration
			}
		}
		data, err := json.Marshal(key)
		if err != nil {
			logger.Log(0, "migration: marshalling enrollment key: "+err.Error())
			continue
		}
		if err = database.Insert(key.Value, string(data), database.ENROLLMENT_KEYS_TABLE_NAME); err != nil {
			logger.Log(0, "migration: inserting enrollment key: "+err.Error())
			continue
		}

	}

	existingKeys, err := logic.GetAllEnrollmentKeys()
	if err != nil {
		return
	}
	// check if any tags are duplicate
	existingTags := make(map[string]struct{})
	for _, existingKey := range existingKeys {
		for _, t := range existingKey.Tags {
			existingTags[t] = struct{}{}
		}
	}
	networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	for _, network := range networks {
		if _, ok := existingTags[network.Name]; ok {
			continue
		}
		_, _ = logic.CreateEnrollmentKey(
			0,
			time.Time{},
			[]string{network.Name},
			[]string{network.Name},
			[]models.TagID{},
			true,
			uuid.Nil,
			true,
			false,
			false,
		)
	}
}

func updateNodes() {
	nodes, err := logic.GetAllNodes()
	if err != nil {
		slog.Error("migration failed for nodes", "error", err)
		return
	}
	for _, node := range nodes {
		node := node
		if node.Tags == nil {
			node.Tags = make(map[models.TagID]struct{})
			logic.UpsertNode(&node)
		}
		if node.IsIngressGateway {
			host := &schema.Host{
				ID: node.HostID,
			}
			err = host.Get(db.WithContext(context.TODO()))
			if err == nil {
				go logic.DeleteRole(models.GetRAGRoleID(node.Network, host.ID.String()), true)
			}
		}
		if node.IsEgressGateway {
			egressRanges, update := removeInterGw(node.EgressGatewayRanges)
			if update {
				node.EgressGatewayRequest.Ranges = egressRanges
				node.EgressGatewayRanges = egressRanges
				logic.UpsertNode(&node)
			}
			if len(node.EgressGatewayRequest.Ranges) > 0 && len(node.EgressGatewayRequest.RangesWithMetric) == 0 {
				for _, egressRangeI := range node.EgressGatewayRequest.Ranges {
					node.EgressGatewayRequest.RangesWithMetric = append(node.EgressGatewayRequest.RangesWithMetric, models.EgressRangeMetric{
						Network:     egressRangeI,
						RouteMetric: 256,
					})
				}
				logic.UpsertNode(&node)
			}

		}
	}
	extclients, _ := logic.GetAllExtClients()
	for _, extclient := range extclients {
		if extclient.Tags == nil {
			extclient.Tags = make(map[models.TagID]struct{})
			logic.SaveExtClient(&extclient)
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

func updateAcls() {
	// get all networks
	if !logic.GetServerSettings().OldAClsSupport {
		return
	}
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		slog.Error("acls migration failed. error getting networks", "error", err)
		return
	}

	// get current acls per network
	for _, network := range networks {
		var networkAcl acls.ACLContainer
		networkAcl, err := networkAcl.Get(acls.ContainerID(network.Name))
		if err != nil {
			if database.IsEmptyRecord(err) {
				continue
			}
			slog.Error(fmt.Sprintf("error during acls migration. error getting acls for network: %s", network.Name), "error", err)
			continue
		}
		// convert old acls to new acls with clients
		// TODO: optimise O(n^2) operation
		clients, err := logic.GetNetworkExtClients(network.Name)
		if err != nil {
			slog.Error(fmt.Sprintf("error during acls migration. error getting clients for network: %s", network.Name), "error", err)
			continue
		}
		clientsIdMap := make(map[string]struct{})
		for _, client := range clients {
			clientsIdMap[client.ClientID] = struct{}{}
		}
		nodeIdsMap := make(map[string]struct{})
		for nodeId := range networkAcl {
			nodeIdsMap[string(nodeId)] = struct{}{}
		}
		/*
			initially, networkACL has only node acls so we add client acls to it
			final shape:
			{
				"node1": {
					"node2": 2,
					"client1": 2,
					"client2": 1,
				},
				"node2": {
					"node1": 2,
					"client1": 2,
					"client2": 1,
				},
				"client1": {
					"node1": 2,
					"node2": 2,
					"client2": 1,
				},
				"client2": {
					"node1": 1,
					"node2": 1,
					"client1": 1,
				},
			}
		*/
		for _, client := range clients {
			networkAcl[acls.AclID(client.ClientID)] = acls.ACL{}
			// add client values to node acls and create client acls with node values
			for id, nodeAcl := range networkAcl {
				// skip if not a node
				if _, ok := nodeIdsMap[string(id)]; !ok {
					continue
				}
				if nodeAcl == nil {
					slog.Warn("acls migration bad data: nil node acl", "node", id, "network", network.Name)
					continue
				}
				nodeAcl[acls.AclID(client.ClientID)] = acls.Allowed
				networkAcl[acls.AclID(client.ClientID)][id] = acls.Allowed
				if client.DeniedACLs == nil {
					continue
				} else if _, ok := client.DeniedACLs[string(id)]; ok {
					nodeAcl[acls.AclID(client.ClientID)] = acls.NotAllowed
					networkAcl[acls.AclID(client.ClientID)][id] = acls.NotAllowed
				}
			}
			// add clients to client acls response
			for _, c := range clients {
				if c.ClientID == client.ClientID {
					continue
				}
				networkAcl[acls.AclID(client.ClientID)][acls.AclID(c.ClientID)] = acls.Allowed
				if client.DeniedACLs == nil {
					continue
				} else if _, ok := client.DeniedACLs[c.ClientID]; ok {
					networkAcl[acls.AclID(client.ClientID)][acls.AclID(c.ClientID)] = acls.NotAllowed
				}
			}
			// delete oneself from its own acl
			delete(networkAcl[acls.AclID(client.ClientID)], acls.AclID(client.ClientID))
		}

		// remove non-existent client and node acls
		for objId := range networkAcl {
			if _, ok := nodeIdsMap[string(objId)]; ok {
				continue
			}
			if _, ok := clientsIdMap[string(objId)]; ok {
				continue
			}
			// remove all occurances of objId from all acls
			for objId2 := range networkAcl {
				delete(networkAcl[objId2], objId)
			}
			delete(networkAcl, objId)
		}

		// save new acls
		slog.Debug(fmt.Sprintf("(migration) saving new acls for network: %s", network.Name), "networkAcl", networkAcl)
		if _, err := networkAcl.Save(acls.ContainerID(network.Name)); err != nil {
			slog.Error(fmt.Sprintf("error during acls migration. error saving new acls for network: %s", network.Name), "error", err)
			continue
		}
		slog.Info(fmt.Sprintf("(migration) successfully saved new acls for network: %s", network.Name))
	}
}

func updateNewAcls() {
	if servercfg.IsPro {
		userGroups, _ := (&schema.UserGroup{}).ListAll(db.WithContext(context.TODO()))
		for _, userGroup := range userGroups {
			_ = logic.EnsureDefaultUserGroupNetworkPolicies(nil, &userGroup, true)
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
	// create default network user roles for existing networks
	if servercfg.IsPro {
		networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
		for _, netI := range networks {
			logic.CreateDefaultNetworkRolesAndGroups(schema.NetworkID(netI.Name))
		}
	}

	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err == nil {
		for _, user := range users {
			user := user
			user.AuthType = schema.BasicAuth
			if logic.IsOauthUser(&user) == nil {
				user.AuthType = schema.OAuth
			}
			if len(user.UserGroups.Data()) == 0 {
				user.UserGroups = datatypes.NewJSONType(make(map[schema.UserGroupID]struct{}))
			}

			logic.AddGlobalNetRolesToAdmins(&user)
			logic.UpsertUser(user)
		}
	}

}

func createDefaultTagsAndPolicies() {
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return
	}
	for _, network := range networks {
		logic.CreateDefaultTags(schema.NetworkID(network.Name))
		logic.CreateDefaultAclNetworkPolicies(schema.NetworkID(network.Name))
		// delete old remote access gws policy
		logic.DeleteAcl(models.Acl{ID: fmt.Sprintf("%s.%s", network.Name, "all-remote-access-gws")})
	}
	logic.MigrateAclPolicies()
	if !servercfg.IsPro {
		nodes, _ := logic.GetAllNodes()
		for _, node := range nodes {
			if node.IsGw {
				node.Tags = make(map[models.TagID]struct{})
				node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
				logic.UpsertNode(&node)
			}
		}
	}
}

func migrateToEgressV1() {
	nodes, _ := logic.GetAllNodes()
	user, err := logic.GetSuperAdmin()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsEgressGateway {
			host := &schema.Host{
				ID: node.HostID,
			}
			err := host.Get(db.WithContext(context.TODO()))
			if err != nil {
				continue
			}
			for _, rangeMetric := range node.EgressGatewayRequest.RangesWithMetric {
				e := &schema.Egress{Range: rangeMetric.Network}
				if err := e.DoesEgressRouteExists(db.WithContext(context.TODO())); err == nil {
					e.Nodes[node.ID.String()] = rangeMetric.RouteMetric
					e.Update(db.WithContext(context.TODO()))
					continue
				}
				e = &schema.Egress{
					ID:          uuid.New().String(),
					Name:        fmt.Sprintf("%s egress", rangeMetric.Network),
					Description: "",
					Network:     node.Network,
					Nodes: datatypes.JSONMap{
						node.ID.String(): rangeMetric.RouteMetric,
					},
					Tags:      make(datatypes.JSONMap),
					Range:     rangeMetric.Network,
					Nat:       node.EgressGatewayRequest.NatEnabled == "yes",
					Status:    true,
					CreatedBy: user.UserName,
					CreatedAt: time.Now().UTC(),
				}
				err = e.Create(db.WithContext(context.TODO()))
				if err == nil {
					acl := models.Acl{
						ID:          uuid.New().String(),
						Name:        "egress node policy",
						MetaData:    "",
						Default:     false,
						ServiceType: models.Any,
						NetworkID:   schema.NetworkID(node.Network),
						Proto:       models.ALL,
						RuleType:    models.DevicePolicy,
						Src: []models.AclPolicyTag{

							{
								ID:    models.NodeTagID,
								Value: "*",
							},
						},
						Dst: []models.AclPolicyTag{
							{
								ID:    models.EgressID,
								Value: e.ID,
							},
						},

						AllowedDirection: models.TrafficDirectionBi,
						Enabled:          true,
						CreatedBy:        "auto",
						CreatedAt:        time.Now().UTC(),
					}
					logic.InsertAcl(acl)
					acl = models.Acl{
						ID:          uuid.New().String(),
						Name:        "egress node policy",
						MetaData:    "",
						Default:     false,
						ServiceType: models.Any,
						NetworkID:   schema.NetworkID(node.Network),
						Proto:       models.ALL,
						RuleType:    models.UserPolicy,
						Src: []models.AclPolicyTag{

							{
								ID:    models.UserAclID,
								Value: "*",
							},
						},
						Dst: []models.AclPolicyTag{
							{
								ID:    models.EgressID,
								Value: e.ID,
							},
						},

						AllowedDirection: models.TrafficDirectionBi,
						Enabled:          true,
						CreatedBy:        "auto",
						CreatedAt:        time.Now().UTC(),
					}
					logic.InsertAcl(acl)
				}

			}
			node.IsEgressGateway = false
			node.EgressGatewayRequest = models.EgressGatewayRequest{}
			node.EgressGatewayNatEnabled = false
			node.EgressGatewayRanges = []string{}
			logic.UpsertNode(&node)

		}
	}
}

func migrateSettings() {
	settingsD := make(map[string]interface{})
	data, err := database.FetchRecord(database.SERVER_SETTINGS, logic.ServerSettingsDBKey)
	if database.IsEmptyRecord(err) {
		logic.UpsertServerSettings(logic.GetServerSettingsFromEnv())
	} else if err == nil {
		json.Unmarshal([]byte(data), &settingsD)
	}
	settings := logic.GetServerSettings()
	if _, ok := settingsD["old_acl_support"]; !ok {
		settings.OldAClsSupport = servercfg.IsOldAclEnabled()
	}
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
	logic.UpsertServerSettings(settings)
}

func deleteOldExtclients() {
	extclients, _ := logic.GetAllExtClients()
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
				_ = logic.DeleteExtClient(extclient.Network, extclient.Network, false)
			}
		}
	}
}

func cleanupDeletedUserGroupRefs() {
	groups, err := (&schema.UserGroup{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return
	}

	existingGroups := make(map[schema.UserGroupID]struct{})
	for _, group := range groups {
		existingGroups[group.ID] = struct{}{}
	}

	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return
	}

	for _, user := range users {
		var update bool
		for groupID := range user.UserGroups.Data() {
			if _, ok := existingGroups[groupID]; !ok {
				delete(user.UserGroups.Data(), groupID)
				update = true
			}
		}

		if update {
			err = user.Update(db.WithContext(context.TODO()))
			if err != nil {
				return
			}
		}
	}

	for _, acl := range logic.ListAcls() {
		var newSrc []models.AclPolicyTag
		var update bool
		for _, src := range acl.Src {
			if src.ID == models.UserGroupAclID {
				if _, ok := existingGroups[schema.UserGroupID(src.Value)]; !ok {
					update = true
				} else {
					newSrc = append(newSrc, src)
				}
			} else {
				newSrc = append(newSrc, src)
			}
		}

		if update {
			if len(newSrc) == 0 {
				err = logic.DeleteAcl(acl)
				if err != nil {
					return
				}
			} else {
				acl.Src = newSrc
				err = logic.UpsertAcl(acl)
				if err != nil {
					return
				}
			}
		}
	}
}
