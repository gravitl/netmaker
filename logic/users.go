package logic

import (
	"encoding/json"
	"sort"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// GetUser - gets a user
// TODO support "masteradmin"
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

// GetReturnUser - gets a user
func GetReturnUser(username string) (models.ReturnUser, error) {

	var user models.ReturnUser
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, username)
	if err != nil {
		return user, err
	}
	if err = json.Unmarshal([]byte(record), &user); err != nil {
		return models.ReturnUser{}, err
	}
	return user, err
}

// ToReturnUser - gets a user as a return user
func ToReturnUser(user models.User) models.ReturnUser {
	return models.ReturnUser{
		UserName:     user.UserName,
		IsSuperAdmin: user.IsSuperAdmin,
		IsAdmin:      user.IsAdmin,
		RemoteGwIDs:  user.RemoteGwIDs,
	}
}

// SetUserDefaults - sets the defaults of a user to avoid empty fields
func SetUserDefaults(user *models.User) {
	if user.RemoteGwIDs == nil {
		user.RemoteGwIDs = make(map[string]struct{})
	}
}

// SortUsers - Sorts slice of Users by username
func SortUsers(unsortedUsers []models.ReturnUser) {
	sort.Slice(unsortedUsers, func(i, j int) bool {
		return unsortedUsers[i].UserName < unsortedUsers[j].UserName
	})
}
