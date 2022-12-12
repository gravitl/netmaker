package pro

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
)

// InitializeNetworkUsers - intializes network users for a given network
func InitializeNetworkUsers(network string) error {

	_, err := database.FetchRecord(database.NETWORK_USER_TABLE_NAME, network)
	if err != nil && database.IsEmptyRecord(err) {
		newNetUserMap := make(promodels.NetworkUserMap)
		netUserData, err := json.Marshal(newNetUserMap)
		if err != nil {
			return err
		}

		return database.Insert(network, string(netUserData), database.NETWORK_USER_TABLE_NAME)
	}
	return err
}

// GetNetworkUsers - gets the network users table
func GetNetworkUsers(network string) (promodels.NetworkUserMap, error) {
	currentUsers, err := database.FetchRecord(database.NETWORK_USER_TABLE_NAME, network)
	if err != nil {
		return nil, err
	}
	var userMap promodels.NetworkUserMap
	if err = json.Unmarshal([]byte(currentUsers), &userMap); err != nil {
		return nil, err
	}
	return userMap, nil
}

// CreateNetworkUser - adds a network user to db
func CreateNetworkUser(network *models.Network, user *promodels.NetworkUser) error {

	if DoesNetworkUserExist(network.NetID, user.ID) {
		return nil
	}

	currentUsers, err := GetNetworkUsers(network.NetID)
	if err != nil {
		return err
	}
	user.SetDefaults()
	currentUsers.Add(user)
	data, err := json.Marshal(currentUsers)
	if err != nil {
		return err
	}

	return database.Insert(network.NetID, string(data), database.NETWORK_USER_TABLE_NAME)
}

// DeleteNetworkUser - deletes a network user and removes from all networks
func DeleteNetworkUser(network, userid string) error {
	currentUsers, err := GetNetworkUsers(network)
	if err != nil {
		return err
	}

	currentUsers.Delete(promodels.NetworkUserID(userid))
	data, err := json.Marshal(currentUsers)
	if err != nil {
		return err
	}

	return database.Insert(network, string(data), database.NETWORK_USER_TABLE_NAME)
}

// DissociateNetworkUserNode - removes a node from a given user's node list
func DissociateNetworkUserNode(userid, networkid, nodeid string) error {
	nuser, err := GetNetworkUser(networkid, promodels.NetworkUserID(userid))
	if err != nil {
		return err
	}
	for i, n := range nuser.Nodes {
		if n == nodeid {
			nuser.Nodes = removeStringIndex(nuser.Nodes, i)
			break
		}
	}
	return UpdateNetworkUser(networkid, nuser)
}

// DissociateNetworkUserClient - removes a client from a given user's client list
func DissociateNetworkUserClient(userid, networkid, clientid string) error {
	nuser, err := GetNetworkUser(networkid, promodels.NetworkUserID(userid))
	if err != nil {
		return err
	}
	for i, n := range nuser.Clients {
		if n == clientid {
			nuser.Clients = removeStringIndex(nuser.Clients, i)
			break
		}
	}
	return UpdateNetworkUser(networkid, nuser)
}

// AssociateNetworkUserClient - removes a client from a given user's client list
func AssociateNetworkUserClient(userid, networkid, clientid string) error {
	nuser, err := GetNetworkUser(networkid, promodels.NetworkUserID(userid))
	if err != nil {
		return err
	}
	var found bool
	for _, n := range nuser.Clients {
		if n == clientid {
			found = true
			break
		}
	}
	if found {
		return nil
	} else {
		nuser.Clients = append(nuser.Clients, clientid)
	}

	return UpdateNetworkUser(networkid, nuser)
}

func removeStringIndex(s []string, index int) []string {
	ret := make([]string, 0)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}

// GetNetworkUser - fetches a network user from a given network
func GetNetworkUser(network string, userID promodels.NetworkUserID) (*promodels.NetworkUser, error) {
	currentUsers, err := GetNetworkUsers(network)
	if err != nil {
		return nil, err
	}
	if currentUsers[userID].ID == "" {
		return nil, fmt.Errorf("user %s does not exist", userID)
	}
	currentNetUser := currentUsers[userID]
	return &currentNetUser, nil
}

// DoesNetworkUserExist - check if networkuser exists
func DoesNetworkUserExist(network string, userID promodels.NetworkUserID) bool {
	_, err := GetNetworkUser(network, userID)
	return err == nil
}

// UpdateNetworkUser - gets a network user from given network
func UpdateNetworkUser(network string, newUser *promodels.NetworkUser) error {
	currentUsers, err := GetNetworkUsers(network)
	if err != nil {
		return err
	}

	currentUsers[newUser.ID] = *newUser
	newUsersData, err := json.Marshal(&currentUsers)
	if err != nil {
		return err
	}

	return database.Insert(network, string(newUsersData), database.NETWORK_USER_TABLE_NAME)
}

// RemoveAllNetworkUsers - removes all network users from given network
func RemoveAllNetworkUsers(network string) error {
	return database.DeleteRecord(database.NETWORK_USER_TABLE_NAME, network)
}

// IsUserNodeAllowed - given a list of nodes, determine if the user's node is allowed based on ID
// Checks if node is in given nodes list as well as being in user's list
func IsUserNodeAllowed(nodes []models.Node, network, userID, nodeID string) bool {

	netUser, err := GetNetworkUser(network, promodels.NetworkUserID(userID))
	if err != nil {
		return false
	}

	for i := range nodes {
		if nodes[i].ID == nodeID {
			for j := range netUser.Nodes {
				if netUser.Nodes[j] == nodeID {
					return true
				}
			}
		}
	}
	return false
}

// IsUserClientAllowed - given a list of clients, determine if the user's client is allowed based on ID
// Checks if client is in given ext client list as well as being in user's list
func IsUserClientAllowed(clients []models.ExtClient, network, userID, clientID string) bool {

	netUser, err := GetNetworkUser(network, promodels.NetworkUserID(userID))
	if err != nil {
		return false
	}

	for i := range clients {
		if clients[i].ClientID == clientID {
			for j := range netUser.Clients {
				if netUser.Clients[j] == clientID {
					return true
				}
			}
		}
	}
	return false
}

// IsUserNetAdmin - checks if a user is a net admin or not
func IsUserNetAdmin(network, userID string) bool {
	user, err := GetNetworkUser(network, promodels.NetworkUserID(userID))
	if err != nil {
		return false
	}
	return user.AccessLevel == NET_ADMIN
}

// MakeNetAdmin - makes a given user a network admin on given network
func MakeNetAdmin(network, userID string) (ok bool) {
	user, err := GetNetworkUser(network, promodels.NetworkUserID(userID))
	if err != nil {
		return ok
	}
	user.AccessLevel = NET_ADMIN
	if err = UpdateNetworkUser(network, user); err != nil {
		return ok
	}
	return true
}

// AssignAccessLvl - gives a user a specified access level
func AssignAccessLvl(network, userID string, accesslvl int) (ok bool) {
	user, err := GetNetworkUser(network, promodels.NetworkUserID(userID))
	if err != nil {
		return ok
	}
	user.AccessLevel = accesslvl
	if err = UpdateNetworkUser(network, user); err != nil {
		return ok
	}
	return true
}
