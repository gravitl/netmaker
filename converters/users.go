package converters

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ToSchemaUser(user models.User) schema.User {
	return schema.User{
		ID:                         "",
		Username:                   user.UserName,
		DisplayName:                user.DisplayName,
		PlatformRoleID:             string(user.PlatformRoleID),
		ExternalIdentityProviderID: user.ExternalIdentityProviderID,
		AccountDisabled:            user.AccountDisabled,
		AuthType:                   string(user.AuthType),
		Password:                   user.Password,
		IsMFAEnabled:               user.IsMFAEnabled,
		TOTPSecret:                 user.TOTPSecret,
		LastLoginAt:                user.LastLoginTime,
		CreatedBy:                  user.CreatedBy,
		CreatedAt:                  user.CreatedAt,
		UpdatedAt:                  user.UpdatedAt,
	}
}

func ToSchemaUsers(users []models.User) []schema.User {
	_users := make([]schema.User, len(users))
	for i, user := range users {
		_users[i] = ToSchemaUser(user)
	}

	return _users
}

func ToModelUser(_user schema.User) models.User {
	return models.User{
		UserName:                   _user.Username,
		ExternalIdentityProviderID: _user.ExternalIdentityProviderID,
		IsMFAEnabled:               _user.IsMFAEnabled,
		TOTPSecret:                 _user.TOTPSecret,
		DisplayName:                _user.DisplayName,
		AccountDisabled:            _user.AccountDisabled,
		Password:                   _user.Password,
		IsAdmin:                    _user.PlatformRoleID == string(models.AdminRole) || _user.PlatformRoleID == string(models.SuperAdminRole),
		IsSuperAdmin:               _user.PlatformRoleID == string(models.SuperAdminRole),
		RemoteGwIDs:                nil,
		AuthType:                   models.AuthType(_user.AuthType),
		UserGroups:                 make(map[models.UserGroupID]struct{}),
		PlatformRoleID:             models.UserRoleID(_user.PlatformRoleID),
		NetworkRoles:               make(map[models.NetworkID]map[models.UserRoleID]struct{}),
		LastLoginTime:              _user.LastLoginAt,
		CreatedBy:                  _user.CreatedBy,
		CreatedAt:                  _user.CreatedAt,
		UpdatedAt:                  _user.UpdatedAt,
	}
}

func ToModelUsers(_users []schema.User) []models.User {
	users := make([]models.User, len(_users))
	for i, user := range _users {
		users[i] = ToModelUser(user)
	}

	return users
}

func ToApiUser(_user schema.User) models.ReturnUser {
	return models.ReturnUser{
		UserName:                   _user.Username,
		ExternalIdentityProviderID: _user.ExternalIdentityProviderID,
		IsMFAEnabled:               _user.IsMFAEnabled,
		DisplayName:                _user.DisplayName,
		AccountDisabled:            _user.AccountDisabled,
		IsAdmin:                    _user.PlatformRoleID == string(models.AdminRole) || _user.PlatformRoleID == string(models.SuperAdminRole),
		IsSuperAdmin:               _user.PlatformRoleID == string(models.SuperAdminRole),
		AuthType:                   models.AuthType(_user.AuthType),
		RemoteGwIDs:                nil,
		UserGroups:                 make(map[models.UserGroupID]struct{}),
		PlatformRoleID:             models.UserRoleID(_user.PlatformRoleID),
		NetworkRoles:               make(map[models.NetworkID]map[models.UserRoleID]struct{}),
		LastLoginTime:              _user.LastLoginAt,
		NumAccessTokens:            0,
		CreatedBy:                  _user.CreatedBy,
		CreatedAt:                  _user.CreatedAt,
		UpdatedAt:                  _user.UpdatedAt,
	}
}

func ToApiUsers(_users []schema.User) []models.ReturnUser {
	users := make([]models.ReturnUser, len(_users))
	for i, user := range _users {
		users[i] = ToApiUser(user)
	}

	return users
}
