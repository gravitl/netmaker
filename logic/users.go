package logic

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

var SyncFromIDP = func() error { return nil }

// GetReturnUser - gets a user
func GetReturnUser(username string) (models.ReturnUser, error) {
	_user := &schema.User{
		Username: username,
	}
	err := _user.Get(db.WithContext(context.TODO()))
	if err != nil {
		return models.ReturnUser{}, err
	}

	return ToReturnUser(_user), nil
}

// ToReturnUser - gets a user as a return user
func ToReturnUser(user *schema.User) models.ReturnUser {
	return models.ReturnUser{
		UserName:                   user.Username,
		ExternalIdentityProviderID: user.ExternalIdentityProviderID,
		IsMFAEnabled:               user.IsMFAEnabled,
		DisplayName:                user.DisplayName,
		AccountDisabled:            user.AccountDisabled,
		IsAdmin:                    user.PlatformRoleID == schema.SuperAdminRole || user.PlatformRoleID == schema.AdminRole,
		IsSuperAdmin:               user.PlatformRoleID == schema.SuperAdminRole,
		AuthType:                   user.AuthType,
		// no need to set. field not in use.
		RemoteGwIDs:    nil,
		UserGroups:     user.UserGroups.Data(),
		PlatformRoleID: user.PlatformRoleID,
		// no need to set. field not in use.
		NetworkRoles:  nil,
		LastLoginTime: user.LastLoginAt,
		CreatedBy:     user.CreatedBy,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

// ToUserEventLog - converts a user to an event log entry with resolved group/role names
func ToUserEventLog(user *schema.User) models.UserEventLog {
	log := models.UserEventLog{
		ReturnUser:          ToReturnUser(user),
		UserGroupsWithNames: make(map[string]string),
	}
	for gID := range user.UserGroups.Data() {
		grp, err := GetUserGroup(gID)
		if err == nil {
			log.UserGroupsWithNames[string(gID)] = grp.Name
		} else {
			log.UserGroupsWithNames[string(gID)] = string(gID)
		}
	}
	return log
}

// SetUserDefaults - sets the defaults of a user to avoid empty fields
func SetUserDefaults(user *schema.User) {
	if len(user.UserGroups.Data()) == 0 {
		user.UserGroups = datatypes.NewJSONType(make(map[schema.UserGroupID]struct{}))
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

	return ToReturnUser(_user), nil
}

func DeletePendingUser(username string) error {
	return (&schema.PendingUser{
		Username: username,
	}).Delete(db.WithContext(context.TODO()))
}

func IsPendingUser(username string) bool {
	exists, err := (&schema.PendingUser{
		Username: username,
	}).Exists(db.WithContext(context.TODO()))
	if err == nil {
		return exists
	}

	return false
}

func GetUserMap() (map[string]schema.User, error) {
	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]schema.User, len(users))
	for _, user := range users {
		userMap[user.Username] = user
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
