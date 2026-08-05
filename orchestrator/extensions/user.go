package extensions

import (
	"context"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

type UserExtensions interface {
	ConfigureAuthType(user *schema.User) error
	ConfigureGlobalAdminGroup(membership *schema.TenantMembership)
	ConfigureGroups(ctx context.Context, membership *schema.TenantMembership, groups datatypes.JSONType[map[schema.UserGroupID]struct{}])
	SendEmailValidation(ctx context.Context, user *schema.User) error
}

type CEUserExtensions struct{}

func (c *CEUserExtensions) ConfigureAuthType(user *schema.User) error {
	user.AuthType = schema.BasicAuth
	return nil
}

func (c *CEUserExtensions) ConfigureGlobalAdminGroup(_ *schema.TenantMembership) {}

func (c *CEUserExtensions) ConfigureGroups(_ context.Context, membership *schema.TenantMembership, _ datatypes.JSONType[map[schema.UserGroupID]struct{}]) {
	membership.Groups = datatypes.NewJSONType(map[schema.UserGroupID]struct{}{})
}

func (c *CEUserExtensions) SendEmailValidation(_ context.Context, _ *schema.User) error {
	return nil
}
