package serverctl

import (
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"golang.org/x/exp/slog"
)

const (
	// NETMAKER_BINARY_NAME - name of netmaker binary
	NETMAKER_BINARY_NAME = "netmaker"
)

func SetDefaults() error {
	if err := setNodeDefaults(); err != nil {
		return err
	}

	if err := setNetworkDefaults(); err != nil {
		return err
	}

	if err := setUserDefaults(); err != nil {
		return err
	}

	return nil
}

// setNodeDefaults - runs through each node and set defaults
func setNodeDefaults() error {
	// upgraded systems will not have ACL's set, which is why we need this function
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	for i := range nodes {
		logic.SetNodeDefaults(&nodes[i], false)
		logic.UpdateNode(&nodes[i], &nodes[i])
		currentNodeACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(nodes[i].Network), nodeacls.NodeID(nodes[i].ID.String()))
		if (err != nil && (database.IsEmptyRecord(err) || strings.Contains(err.Error(), "no node ACL present"))) || currentNodeACL == nil {
			if _, err = nodeacls.CreateNodeACL(nodeacls.NetworkID(nodes[i].Network), nodeacls.NodeID(nodes[i].ID.String()), acls.Allowed); err != nil {
				logger.Log(1, "could not create a default ACL for node", nodes[i].ID.String())
			}
		}
	}
	return nil
}

func setNetworkDefaults() error {
	// upgraded systems will not have NetworkUsers's set, which is why we need this function
	networks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	for _, network := range networks {
		if network.SetDefaults() {
			logic.SaveNetwork(&network)
		}
	}
	return nil
}

func setUserDefaults() error {
	users, err := logic.GetUsers()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	for _, user := range users {
		updateUser, err := logic.GetUser(user.UserName)
		if err != nil {
			slog.Error("could not get user", "user", updateUser.UserName, "error", err.Error())
		}
		logic.SetUserDefaults(updateUser)
		err = logic.UpsertUser(*updateUser)
		if err != nil {
			slog.Error("could not update user", "user", updateUser.UserName, "error", err.Error())
		}
	}
	return nil
}
