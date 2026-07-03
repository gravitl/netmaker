package extensions

import (
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

type UserExtensions interface {
	ConfigureAuthType(user *schema.User) error
	ConfigureGlobalAdminGroup(membership *schema.TenantMembership)
	ConfigureGroups(membership *schema.TenantMembership, groups datatypes.JSONType[map[schema.UserGroupID]struct{}])
}

type CEUserExtensions struct{}

func (c *CEUserExtensions) ConfigureAuthType(user *schema.User) error {
	user.AuthType = schema.BasicAuth
	return nil
}

func (c *CEUserExtensions) ConfigureGlobalAdminGroup(_ *schema.TenantMembership) {}

func (c *CEUserExtensions) ConfigureGroups(membership *schema.TenantMembership, _ datatypes.JSONType[map[schema.UserGroupID]struct{}]) {
	membership.Groups = datatypes.NewJSONType(map[schema.UserGroupID]struct{}{})
}
