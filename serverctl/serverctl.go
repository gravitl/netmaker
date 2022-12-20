package serverctl

import (
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/logic/pro"
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
		logic.SetNodeDefaults(&nodes[i])
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
	for _, net := range networks {
		if err = pro.InitializeNetworkUsers(net.NetID); err != nil {
			logger.Log(0, "could not initialize NetworkUsers on network", net.NetID)
		}
		pro.AddProNetDefaults(&net)
		update := false
		newNet := net
		if strings.Contains(net.NetID, ".") {
			newNet.NetID = strings.ReplaceAll(net.NetID, ".", "")
			newNet.DefaultInterface = strings.ReplaceAll(net.DefaultInterface, ".", "")
			update = true
		}
		if strings.ContainsAny(net.NetID, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			newNet.NetID = strings.ToLower(net.NetID)
			newNet.DefaultInterface = strings.ToLower(net.DefaultInterface)
			update = true
		}
		if update {
			newNet.SetDefaults()
			if err := logic.SaveNetwork(&newNet); err != nil {
				logger.Log(0, "error saving networks during initial update:", err.Error())
			}
			if err := logic.DeleteNetwork(net.NetID); err != nil {
				logger.Log(0, "error deleting old network:", err.Error())
			}
		} else {
			net.SetDefaults()
			_, _, _, _, _, _, err = logic.UpdateNetwork(&net, &net)
			if err != nil {
				logger.Log(0, "could not set defaults on network", net.NetID)
			}
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
			logger.Log(0, "could not update user", updateUser.UserName)
		}
		logic.SetUserDefaults(&updateUser)
		copyUser := updateUser
		copyUser.Password = ""
		if _, err = logic.UpdateUser(copyUser, updateUser); err != nil {
			logger.Log(0, "could not update user", updateUser.UserName)
		}
	}
	return nil
}
