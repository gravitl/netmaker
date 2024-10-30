package logic

import (
	"encoding/json"
	"errors"
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
		UserName:       user.UserName,
		PlatformRoleID: user.PlatformRoleID,
		AuthType:       user.AuthType,
		UserGroups:     user.UserGroups,
		NetworkRoles:   user.NetworkRoles,
		RemoteGwIDs:    user.RemoteGwIDs,
		LastLoginTime:  user.LastLoginTime,
	}
}

// SetUserDefaults - sets the defaults of a user to avoid empty fields
func SetUserDefaults(user *models.User) {
	if user.RemoteGwIDs == nil {
		user.RemoteGwIDs = make(map[string]struct{})
	}
	if len(user.NetworkRoles) == 0 {
		user.NetworkRoles = make(map[models.NetworkID]map[models.UserRoleID]struct{})
	}
	if len(user.UserGroups) == 0 {
		user.UserGroups = make(map[models.UserGroupID]struct{})
	}
	AddGlobalNetRolesToAdmins(user)
}

// SortUsers - Sorts slice of Users by username
func SortUsers(unsortedUsers []models.ReturnUser) {
	sort.Slice(unsortedUsers, func(i, j int) bool {
		return unsortedUsers[i].UserName < unsortedUsers[j].UserName
	})
}

// GetSuperAdmin - fetches superadmin user
func GetSuperAdmin() (models.ReturnUser, error) {
	users, err := GetUsers()
	if err != nil {
		return models.ReturnUser{}, err
	}
	for _, user := range users {
		if user.IsSuperAdmin {
			return user, nil
		}
	}
	return models.ReturnUser{}, errors.New("superadmin not found")
}

func InsertPendingUser(u *models.User) error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return database.Insert(u.UserName, string(data), database.PENDING_USERS_TABLE_NAME)
}

func DeletePendingUser(username string) error {
	return database.DeleteRecord(database.PENDING_USERS_TABLE_NAME, username)
}

func IsPendingUser(username string) bool {
	records, err := database.FetchRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil {
		return false

	}
	for _, record := range records {
		u := models.ReturnUser{}
		err := json.Unmarshal([]byte(record), &u)
		if err == nil && u.UserName == username {
			return true
		}
	}
	return false
}

func ListPendingUsers() ([]models.ReturnUser, error) {
	pendingUsers := []models.ReturnUser{}
	records, err := database.FetchRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return pendingUsers, err
	}
	for _, record := range records {
		u := models.ReturnUser{}
		err = json.Unmarshal([]byte(record), &u)
		if err == nil {
			pendingUsers = append(pendingUsers, u)
		}
	}
	return pendingUsers, nil
}

func GetUserMap() (map[string]models.User, error) {
	userMap := make(map[string]models.User)
	records, err := database.FetchRecords(database.USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return userMap, err
	}
	for _, record := range records {
		u := models.User{}
		err = json.Unmarshal([]byte(record), &u)
		if err == nil {
			userMap[u.UserName] = u
		}
	}
	return userMap, nil
}

func InsertUserInvite(invite models.UserInvite) error {
	data, err := json.Marshal(invite)
	if err != nil {
		return err
	}
	return database.Insert(invite.Email, string(data), database.USER_INVITES_TABLE_NAME)
}

func GetUserInvite(email string) (in models.UserInvite, err error) {
	d, err := database.FetchRecord(database.USER_INVITES_TABLE_NAME, email)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(d), &in)
	return
}

func ListUserInvites() ([]models.UserInvite, error) {
	invites := []models.UserInvite{}
	records, err := database.FetchRecords(database.USER_INVITES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return invites, err
	}
	for _, record := range records {
		in := models.UserInvite{}
		err = json.Unmarshal([]byte(record), &in)
		if err == nil {
			invites = append(invites, in)
		}
	}
	return invites, nil
}

func DeleteUserInvite(email string) error {
	return database.DeleteRecord(database.USER_INVITES_TABLE_NAME, email)
}
func ValidateAndApproveUserInvite(email, code string) error {
	in, err := GetUserInvite(email)
	if err != nil {
		return err
	}
	if code != in.InviteCode {
		return errors.New("invalid code")
	}
	return nil
}
