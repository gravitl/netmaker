package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
)

// GetUser - gets a user
func GetUser(username string) (*models.User, error) {

	var user models.User
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, username)
	if err != nil {
		return &user, err
	}
	if err = json.Unmarshal([]byte(record), &user); err != nil {
		return &models.User{}, err
	}
	return &user, err
}

// GetGroupUsers - gets users in a group
func GetGroupUsers(group string) ([]models.ReturnUser, error) {
	var returnUsers []models.ReturnUser
	users, err := GetUsers()
	if err != nil {
		return returnUsers, err
	}
	for _, user := range users {
		if StringSliceContains(user.Groups, group) {
			users = append(users, user)
		}
	}
	return users, err
}

// == PRO ==

// InitializeNetUsers - intializes network users for all users/networks
func InitializeNetUsers(network *models.Network) error {
	// == add all current users to network as network users ==
	currentUsers, err := GetUsers()
	if err != nil {
		return err
	}

	for i := range currentUsers { // add all users to given network
		newUser := promodels.NetworkUser{
			ID:          promodels.NetworkUserID(currentUsers[i].UserName),
			Clients:     []string{},
			Nodes:       []string{},
			AccessLevel: pro.NO_ACCESS,
			ClientLimit: 0,
			NodeLimit:   0,
		}
		if pro.IsUserAllowed(network, currentUsers[i].UserName, currentUsers[i].Groups) {
			newUser.AccessLevel = network.ProSettings.DefaultAccessLevel
			newUser.ClientLimit = network.ProSettings.DefaultUserClientLimit
			newUser.NodeLimit = network.ProSettings.DefaultUserNodeLimit
		}

		if err = pro.CreateNetworkUser(network, &newUser); err != nil {
			logger.Log(0, "failed to add network user settings to user", string(newUser.ID), "on network", network.NetID)
		}
	}
	return nil
}

// SetUserDefaults - sets the defaults of a user to avoid empty fields
func SetUserDefaults(user *models.User) {
	if user.Groups == nil {
		user.Groups = []string{pro.DEFAULT_ALLOWED_GROUPS}
	}
}
