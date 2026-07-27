package extensions

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/logic"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProUserExtensions struct{}

func (p *ProUserExtensions) ConfigureAuthType(user *schema.User) error {
	user.AuthType = schema.BasicAuth
	oauthSecret, err := logic.FetchOAuthSecret()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}

		return err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oauthSecret))
	if err == nil {
		user.AuthType = schema.OAuth
	}

	return nil
}

func (p *ProUserExtensions) ConfigureGlobalAdminGroup(membership *schema.TenantMembership) {
	if membership.RoleID == schema.SuperAdminRole || membership.RoleID == schema.AdminRole {
		if len(membership.Groups.Data()) == 0 {
			membership.Groups = datatypes.NewJSONType(make(map[schema.UserGroupID]struct{}))
			membership.Groups.Data()[proLogic.GetDefaultGlobalAdminGroupID()] = struct{}{}
		}
	}
}

func (p *ProUserExtensions) ConfigureGroups(ctx context.Context, membership *schema.TenantMembership, groups datatypes.JSONType[map[schema.UserGroupID]struct{}]) {
	membership.Groups = datatypes.NewJSONType(make(map[schema.UserGroupID]struct{}))
	for groupID := range groups.Data() {
		_, err := proLogic.GetUserGroup(ctx, groupID)
		if err == nil {
			membership.Groups.Data()[groupID] = struct{}{}
		}
	}
}
