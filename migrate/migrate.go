package migrate

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"golang.org/x/exp/slog"
	"gorm.io/datatypes"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

// Run - runs all migrations
func Run() {
	updateEnrollmentKeys()
	assignSuperAdmin()
	createDefaultTagsAndPolicies()
	removeOldUserGrps()
	syncUsers()
	updateHosts()
	updateNodes()
	updateAcls()
	migrateToGws()
	migrateToEgressV1()
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
		user, err := logic.GetUser(owner)
		if err != nil {
			log.Fatal("error getting user", "user", owner, "error", err.Error())
		}
		user.PlatformRoleID = models.SuperAdminRole
		err = logic.UpsertUser(*user)
		if err != nil {
			log.Fatal(
				"error updating user to superadmin",
				"user",
				user.UserName,
				"error",
				err.Error(),
			)
		}
		return
	}
	for _, u := range users {
		if u.IsAdmin {
			user, err := logic.GetUser(u.UserName)
			if err != nil {
				slog.Error("error getting user", "user", u.UserName, "error", err.Error())
				continue
			}
			user.PlatformRoleID = models.SuperAdminRole
			user.IsSuperAdmin = true
			err = logic.UpsertUser(*user)
			if err != nil {
				slog.Error(
					"error updating user to superadmin",
					"user",
					user.UserName,
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
	networks, _ := logic.GetNetworks()
	for _, network := range networks {
		if _, ok := existingTags[network.NetID]; ok {
			continue
		}
		_, _ = logic.CreateEnrollmentKey(
			0,
			time.Time{},
			[]string{network.NetID},
			[]string{network.NetID},
			[]models.TagID{},
			true,
			uuid.Nil,
			true,
			false,
		)

	}
}

func removeOldUserGrps() {
	rows, err := database.FetchRecords(database.USER_GROUPS_TABLE_NAME)
	if err != nil {
		return
	}
	for key, row := range rows {
		userG := models.UserGroup{}
		_ = json.Unmarshal([]byte(row), &userG)
		if userG.ID == "" {
			database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, key)
		}
	}
}

func updateHosts() {
	rows, err := database.FetchRecords(database.HOSTS_TABLE_NAME)
	if err != nil {
		logger.Log(0, "failed to fetch database records for hosts")
	}
	for _, row := range rows {
		var host models.Host
		if err := json.Unmarshal([]byte(row), &host); err != nil {
			logger.Log(0, "failed to unmarshal database row to host", "row", row)
			continue
		}
		if host.PersistentKeepalive == 0 {
			host.PersistentKeepalive = models.DefaultPersistentKeepAlive
			if err := logic.UpsertHost(&host); err != nil {
				logger.Log(0, "failed to upsert host", host.ID.String())
				continue
			}
		}
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
			host, err := logic.GetHost(node.HostID.String())
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
	networks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		slog.Error("acls migration failed. error getting networks", "error", err)
		return
	}

	// get current acls per network
	for _, network := range networks {
		var networkAcl acls.ACLContainer
		networkAcl, err := networkAcl.Get(acls.ContainerID(network.NetID))
		if err != nil {
			if database.IsEmptyRecord(err) {
				continue
			}
			slog.Error(fmt.Sprintf("error during acls migration. error getting acls for network: %s", network.NetID), "error", err)
			continue
		}
		// convert old acls to new acls with clients
		// TODO: optimise O(n^2) operation
		clients, err := logic.GetNetworkExtClients(network.NetID)
		if err != nil {
			slog.Error(fmt.Sprintf("error during acls migration. error getting clients for network: %s", network.NetID), "error", err)
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
					slog.Warn("acls migration bad data: nil node acl", "node", id, "network", network.NetID)
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
		slog.Debug(fmt.Sprintf("(migration) saving new acls for network: %s", network.NetID), "networkAcl", networkAcl)
		if _, err := networkAcl.Save(acls.ContainerID(network.NetID)); err != nil {
			slog.Error(fmt.Sprintf("error during acls migration. error saving new acls for network: %s", network.NetID), "error", err)
			continue
		}
		slog.Info(fmt.Sprintf("(migration) successfully saved new acls for network: %s", network.NetID))
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
	// create default network user roles for existing networks
	if servercfg.IsPro {
		networks, _ := logic.GetNetworks()
		for _, netI := range networks {
			logic.CreateDefaultNetworkRolesAndGroups(models.NetworkID(netI.NetID))
		}
	}

	users, err := logic.GetUsersDB()
	if err == nil {
		for _, user := range users {
			user := user
			if user.PlatformRoleID == models.AdminRole && !user.IsAdmin {
				user.IsAdmin = true
				logic.UpsertUser(user)
			}
			if user.PlatformRoleID == models.SuperAdminRole && !user.IsSuperAdmin {
				user.IsSuperAdmin = true
				logic.UpsertUser(user)
			}
			if user.PlatformRoleID.String() != "" {
				logic.MigrateUserRoleAndGroups(user)
				logic.AddGlobalNetRolesToAdmins(user)
				continue
			}
			user.AuthType = models.BasicAuth
			if logic.IsOauthUser(&user) == nil {
				user.AuthType = models.OAuth
			}
			if len(user.NetworkRoles) == 0 {
				user.NetworkRoles = make(map[models.NetworkID]map[models.UserRoleID]struct{})
			}
			if len(user.UserGroups) == 0 {
				user.UserGroups = make(map[models.UserGroupID]struct{})
			}
			if user.IsSuperAdmin {
				user.PlatformRoleID = models.SuperAdminRole

			} else if user.IsAdmin {
				user.PlatformRoleID = models.AdminRole
			} else {
				user.PlatformRoleID = models.ServiceUser
			}
			logic.UpsertUser(user)
			logic.AddGlobalNetRolesToAdmins(user)
			logic.MigrateUserRoleAndGroups(user)
		}
	}

}

func createDefaultTagsAndPolicies() {
	networks, err := logic.GetNetworks()
	if err != nil {
		return
	}
	for _, network := range networks {
		logic.CreateDefaultTags(models.NetworkID(network.NetID))
		logic.CreateDefaultAclNetworkPolicies(models.NetworkID(network.NetID))
		// delete old remote access gws policy
		logic.DeleteAcl(models.Acl{ID: fmt.Sprintf("%s.%s", network.NetID, "all-remote-access-gws")})
	}
	logic.MigrateAclPolicies()
}

func migrateToGws() {
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsIngressGateway || node.IsRelay {
			node.IsGw = true
			node.IsIngressGateway = true
			node.IsRelay = true
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
			delete(node.Tags, models.TagID(fmt.Sprintf("%s.%s", node.Network, models.OldRemoteAccessTagName)))
			logic.UpsertNode(&node)
		}
	}
	acls := logic.ListAcls()
	for _, acl := range acls {
		upsert := false
		for i, srcI := range acl.Src {
			if srcI.ID == models.NodeTagID && srcI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				srcI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Src[i] = srcI
				upsert = true
			}
		}
		for i, dstI := range acl.Dst {
			if dstI.ID == models.NodeTagID && dstI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				dstI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Dst[i] = dstI
				upsert = true
			}
		}
		if upsert {
			logic.UpsertAcl(acl)
		}
	}
	nets, _ := logic.GetNetworks()
	for _, netI := range nets {
		logic.DeleteTag(models.TagID(fmt.Sprintf("%s.%s", netI.NetID, models.OldRemoteAccessTagName)), true)
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
			for _, rangeI := range node.EgressGatewayRequest.Ranges {
				e := models.Egress{
					ID:          uuid.New().String(),
					Name:        rangeI,
					Description: "add description",
					Network:     node.Network,
					Nodes: datatypes.JSONMap{
						node.ID.String(): 256,
					},
					Tags:      make(datatypes.JSONMap),
					Range:     rangeI,
					Nat:       node.EgressGatewayRequest.NatEnabled == "yes",
					CreatedBy: user.UserName,
					CreatedAt: time.Now().UTC(),
				}
				err = e.Create()
				if err == nil {
					node.IsEgressGateway = false
					node.EgressGatewayRequest = models.EgressGatewayRequest{}
					node.EgressGatewayNatEnabled = false
					node.EgressGatewayRanges = []string{}
					logic.UpsertNode(&node)
				}
			}
		}
		if node.IsInternetGateway {
			e := models.Egress{
				ID:          uuid.New().String(),
				Name:        "inet gw",
				Description: "add description",
				Network:     node.Network,
				Nodes: datatypes.JSONMap{
					node.ID.String(): 256,
				},
				Tags:      make(datatypes.JSONMap),
				Range:     "",
				IsInetGw:  true,
				Nat:       node.EgressGatewayRequest.NatEnabled == "yes",
				CreatedBy: user.UserName,
				CreatedAt: time.Now().UTC(),
			}
			err = e.Create()
			if err == nil {
				node.IsEgressGateway = false
				node.EgressGatewayRequest = models.EgressGatewayRequest{}
				node.EgressGatewayNatEnabled = false
				node.EgressGatewayRanges = []string{}
				node.IsInternetGateway = false
				src := []models.AclPolicyTag{}
				for _, inetClientID := range node.InetNodeReq.InetNodeClientIDs {
					_, err := logic.GetNodeByID(inetClientID)
					if err == nil {
						src = append(src, models.AclPolicyTag{
							ID:    models.NodeID,
							Value: inetClientID,
						})
					}
				}
				acl := models.Acl{
					ID:          uuid.New().String(),
					Name:        "exit node policy",
					MetaData:    "all traffic on source nodes will pass through the destination node in the policy",
					Default:     false,
					ServiceType: models.Any,
					NetworkID:   models.NetworkID(node.Network),
					Proto:       models.ALL,
					RuleType:    models.UserPolicy,
					Src:         src,
					Dst: []models.AclPolicyTag{
						{
							ID:    models.NodeID,
							Value: node.ID.String(),
						}},
					AllowedDirection: models.TrafficDirectionBi,
					Enabled:          true,
					CreatedBy:        "auto",
					CreatedAt:        time.Now().UTC(),
				}
				logic.InsertAcl(acl)
				node.InetNodeReq = models.InetNodeReq{}
				logic.UpsertNode(&node)
			}
		}
		if node.InternetGwID != "" {
			node.InternetGwID = ""
			logic.UpsertNode(&node)
		}
	}
}
