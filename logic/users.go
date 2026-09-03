package logic

import (
	"context"
	"errors"
	"sort"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

var SyncFromIDP = func(context.Context) error { return nil }

var ErrUserLimitExceeded = errors.New("user limit reached for this tenant, please upgrade your license")

var UserLimitExceeded = func(ctx context.Context) bool {
	return false
}

// ToReturnUser - gets a user as a return user
func ToReturnUser(user *schema.User) models.ReturnUser {
	return models.ReturnUser{
		UserName:                   user.Username,
		ExternalIdentityProviderID: user.ExternalIdentityProviderID,
		IsMFAEnabled:               user.IsMFAEnabled,
		EmailValidated:             user.EmailValidated,
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
func ToUserEventLog(ctx context.Context, user *schema.User) models.UserEventLog {
	log := models.UserEventLog{
		ReturnUser:          ToReturnUser(user),
		UserGroupsWithNames: make(map[string]string),
	}
	for gID := range user.UserGroups.Data() {
		grp, err := GetUserGroup(ctx, gID)
		if err == nil {
			log.UserGroupsWithNames[string(gID)] = grp.Name
		} else {
			log.UserGroupsWithNames[string(gID)] = string(gID)
		}
	}
	return log
}

// SortUsers - Sorts slice of Users by username
func SortUsers(unsortedUsers []models.ReturnUser) {
	sort.Slice(unsortedUsers, func(i, j int) bool {
		return unsortedUsers[i].UserName < unsortedUsers[j].UserName
	})
}

// GetSuperAdmin - fetches superadmin user
func GetSuperAdmin(ctx context.Context) (models.ReturnUser, error) {
	_user := &schema.User{}
	err := _user.GetSuperAdmin(ctx)
	if err != nil {
		return models.ReturnUser{}, err
	}

	return ToReturnUser(_user), nil
}

func IsPendingUser(ctx context.Context, username string) bool {
	exists, err := (&schema.PendingUser{
		Username: username,
	}).Exists(ctx)
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

func GetUserInvite(ctx context.Context, email string) (*schema.UserInvite, error) {
	userInvite := &schema.UserInvite{
		Email: email,
	}
	err := userInvite.GetByEmail(ctx)
	if err != nil {
		return nil, err
	}

	return userInvite, nil
}

func ValidateAndApproveUserInvite(ctx context.Context, email, code string) error {
	in, err := GetUserInvite(ctx, email)
	if err != nil {
		return err
	}
	if code != in.InviteCode {
		return errors.New("invalid code")
	}
	return nil
}
