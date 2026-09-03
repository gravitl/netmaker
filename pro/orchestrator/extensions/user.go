package extensions

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/email"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProUserExtensions struct{}

func (p *ProUserExtensions) ConfigureAuthType(ctx context.Context, user *schema.User) error {
	user.AuthType = schema.BasicAuth
	oauthSecret, err := logic.FetchOAuthSecret(ctx)
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

func (p *ProUserExtensions) SendEmailValidation(ctx context.Context, user *schema.User) error {
	invite := &schema.UserInvite{Email: user.Username}
	err := invite.GetPendingValidationInvite(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		inviteCode := logic.RandomString(8)
		validateURL := fmt.Sprintf(
			"https://api.%s/api/v1/users/validate-email?invite_code=%s&email=%s",
			servercfg.GetNmBaseDomain(),
			url.QueryEscape(inviteCode),
			url.QueryEscape(user.Username),
		)

		invite = &schema.UserInvite{
			Scope:      scope.GlobalScope,
			InviteCode: inviteCode,
			InviteURL:  validateURL,
			Email:      user.Username,
			Type:       schema.ValidationInvite,
		}
		err = invite.Create(ctx)
		if err != nil {
			return err
		}
	}

	e := email.EmailValidationMail{
		BodyBuilder: &email.EmailBodyBuilderWithH1HeadlineAndImage{},
		ValidateURL: invite.InviteURL,
	}
	n := email.Notification{RecipientMail: user.Username}
	err = email.GetClient().SendEmail(ctx, n, e)
	if err != nil {
		logger.Log(2, "failed to send email validation email:", err.Error())
	}

	return nil
}
