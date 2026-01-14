package logic

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// GetUser - gets a user
// TODO support "masteradmin"
func GetUser(username string) (*models.User, error) {
	_user := schema.User{
		Username: username,
	}
	err := _user.Get(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	user := converters.ToModelUser(_user)

	user.UserGroups, user.NetworkRoles, err = GetUserGroupsAndNetworkRoles(_user.ID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetReturnUser - gets a user
func GetReturnUser(username string) (models.ReturnUser, error) {
	_user := schema.User{
		Username: username,
	}
	err := _user.Get(db.WithContext(context.TODO()))
	if err != nil {
		return models.ReturnUser{}, err
	}

	user := converters.ToApiUser(_user)

	user.UserGroups, user.NetworkRoles, err = GetUserGroupsAndNetworkRoles(_user.ID)
	if err != nil {
		return models.ReturnUser{}, err
	}

	return user, nil
}

func GetUserGroupsAndNetworkRoles(_userID string) (map[models.UserGroupID]struct{}, map[models.NetworkID]map[models.UserRoleID]struct{}, error) {
	userGroups := make(map[models.UserGroupID]struct{})
	networkRoles := make(map[models.NetworkID]map[models.UserRoleID]struct{})

	_userGroup := schema.Memberships{
		UserID: _userID,
	}
	_userGroups, err := _userGroup.ListAllMemberships(db.WithContext(context.TODO()))
	if err != nil {
		return nil, nil, err
	}

	for _, _group := range _userGroups {
		userGroups[models.UserGroupID(_group.GroupID)] = struct{}{}
	}

	_networkRole := schema.UserNetworkRole{
		UserID: _userID,
	}
	_networkRoles, err := _networkRole.ListAllNetworkRoles(db.WithContext(context.TODO()))
	if err != nil {
		return nil, nil, err
	}

	for _, _role := range _networkRoles {
		networkRoles[models.NetworkID(_role.NetworkID)] = map[models.UserRoleID]struct{}{
			models.UserRoleID(_role.RoleID): {},
		}
	}

	return userGroups, networkRoles, nil
}

// ToReturnUser - gets a user as a return user
func ToReturnUser(user models.User) models.ReturnUser {
	return models.ReturnUser{
		UserName:                   user.UserName,
		ExternalIdentityProviderID: user.ExternalIdentityProviderID,
		IsMFAEnabled:               user.IsMFAEnabled,
		DisplayName:                user.DisplayName,
		AccountDisabled:            user.AccountDisabled,
		AuthType:                   user.AuthType,
		RemoteGwIDs:                user.RemoteGwIDs,
		UserGroups:                 user.UserGroups,
		PlatformRoleID:             user.PlatformRoleID,
		NetworkRoles:               user.NetworkRoles,
		LastLoginTime:              user.LastLoginTime,
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
}

// SortUsers - Sorts slice of Users by username
func SortUsers(unsortedUsers []models.ReturnUser) {
	sort.Slice(unsortedUsers, func(i, j int) bool {
		return unsortedUsers[i].UserName < unsortedUsers[j].UserName
	})
}

// GetSuperAdmin - fetches superadmin user
func GetSuperAdmin() (models.ReturnUser, error) {
	_user := &schema.User{}
	err := _user.GetSuperAdmin(db.WithContext(context.TODO()))
	if err != nil {
		return models.ReturnUser{}, err
	}

	return converters.ToApiUser(*_user), nil
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

func ListPendingReturnUsers() ([]models.ReturnUser, error) {
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

func ListPendingUsers() ([]models.User, error) {
	var pendingUsers []models.User
	records, err := database.FetchRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return pendingUsers, err
	}
	for _, record := range records {
		var u models.User
		err = json.Unmarshal([]byte(record), &u)
		if err == nil {
			pendingUsers = append(pendingUsers, u)
		}
	}
	return pendingUsers, nil
}

func GetUserMap() (map[string]models.User, error) {
	users, err := GetUsersDB()
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]models.User)
	for _, user := range users {
		userMap[user.UserName] = user
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
