package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const OIDC_TIMEOUT = 10 * time.Second

// OIDCProvider implements Provider for OIDC-compatible identity providers (including Okta).
type OIDCProvider struct {
	name     string
	cfg      *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewOIDCProvider constructs an OIDCProvider by performing OIDC discovery against the issuer URL.
func NewOIDCProvider(redirectURL, clientID, clientSecret, issuer string) (*OIDCProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), OIDC_TIMEOUT)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("initializing OIDC provider with issuer %q: %w", issuer, err)
	}

	return &OIDCProvider{
		name:     oidc_provider_name,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

func (p *OIDCProvider) Name() string {
	return p.name
}

func (p *OIDCProvider) Config() *oauth2.Config {
	return p.cfg
}

func (p *OIDCProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	appName := r.Header.Get("X-Application-Name")
	if appName == "" {
		appName = logic.NetmakerDesktopApp
	}

	oauthState := logic.RandomString(user_signin_length)
	err := logic.SetState(scope.Level(r.Context()), scope.ID(r.Context()), appName, oauthState)
	if err != nil {
		handleSomethingWentWrong(w)
		return
	}

	url := p.cfg.AuthCodeURL(oauthState)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (p *OIDCProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var rState, rCode = getStateAndCode(r)

	state, err := logic.GetState(rState)
	if err != nil {
		handleOauthNotValid(w)
		return
	}

	content, err := p.GetUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from callback:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}

	var inviteExists bool
	in, err := logic.GetUserInvite(r.Context(), content.Email)
	if err == nil {
		inviteExists = true
	}
	if !inviteExists && logic.IsPendingUser(r.Context(), content.Email) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}

	user := &schema.User{Username: content.Email}
	err = user.GetWithMembership(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if inviteExists {
				user, err := proLogic.PrepareOauthUserFromInvite(r.Context(), in)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
					return
				}
				user.ExternalIdentityProviderID = string(content.ID)
				err = orchestrator.GetRepository().UserOrchestrator().CreateUser(r.Context(), &user)
				if err != nil {
					if errors.Is(err, logic.ErrUserLimitExceeded) {
						handleUserLimitExceeded(w)
						return
					}
					handleSomethingWentWrong(w)
					return
				}
				_ = (&schema.UserInvite{
					Email: user.Username,
				}).DeleteByEmail(r.Context())

				_ = (&schema.PendingUser{
					Username: content.Email,
				}).Delete(r.Context())
			} else {
				if scope.Level(r.Context()) == scope.OrgScope {
					handleOauthInviteNotFound(w)
					return
				}

				if !isEmailAllowed(r.Context(), content.Email) {
					handleOauthUserNotAllowedToSignUp(w)
					return
				}
				pendingUser := &schema.PendingUser{
					Scope:                      scope.Level(r.Context()),
					ScopeID:                    scope.ID(r.Context()),
					Username:                   content.Email,
					ExternalIdentityProviderID: string(content.ID),
				}
				err = pendingUser.Create(r.Context())
				if err != nil {
					handleSomethingWentWrong(w)
					return
				}
				handleFirstTimeOauthUserSignUp(w)
				return
			}
		} else {
			handleSomethingWentWrong(w)
			return
		}
	} else {
		if user.AuthType == schema.BasicAuth {
			logger.Log(0, "invalid auth type: basic_auth")
			handleAuthTypeMismatch(w)
			return
		}
	}

	user = &schema.User{Username: content.Email}
	err = user.GetWithMembership(r.Context())
	if err != nil {
		handleOauthUserNotFound(w)
		return
	}

	if user.AccountDisabled {
		handleUserAccountDisabled(w)
		return
	}

	userRole := &schema.UserRole{ID: user.PlatformRoleID}
	err = userRole.GetPlatformRole(r.Context())
	if err != nil {
		handleSomethingWentWrong(w)
		return
	}
	if userRole.DenyDashboardAccess {
		handleOauthUserNotAllowed(w)
		return
	}

	newPass, fetchErr := logic.FetchOAuthSecret(r.Context())
	if fetchErr != nil {
		return
	}
	authRequest := models.UserAuthParams{
		UserName: content.Email,
		Password: newPass,
	}
	jwt, jwtErr := logic.VerifyAuthRequest(r.Context(), authRequest, state.AppName)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName, jwtErr.Error())
		return
	}

	logic.LogEvent(r.Context(), &models.Event{
		Action: schema.Login,
		Source: models.Subject{
			ID:   user.Username,
			Name: user.Username,
			Type: schema.UserSub,
		},
		TriggeredBy: user.Username,
		Target: models.Subject{
			ID:   schema.DashboardSub.String(),
			Name: schema.DashboardSub.String(),
			Type: schema.DashboardSub,
			Info: logic.ToReturnUser(user),
		},
		Origin: schema.Dashboard,
	})
	logger.Log(1, "completed OIDC OAuth signin for", content.Email)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.Email, http.StatusPermanentRedirect)
}

func (p *OIDCProvider) GetUserInfo(state string, code string) (u *OAuthUser, e error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	logger.Log(3, "using oauth state string:", oauth_state_string)
	logger.Log(3, "            state string:", state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}

	defer func() {
		if r := recover(); r != nil {
			e = fmt.Errorf("GetUserInfo panic: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), OIDC_TIMEOUT)
	defer cancel()

	oauth2Token, err := p.cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange oauth2 token using code %q", code)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("failed to get raw id_token from oauth2 token")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify raw id_token: %w", err)
	}

	u = &OAuthUser{}
	if err := idToken.Claims(u); err != nil {
		e = fmt.Errorf("error when claiming OIDCUser: %w", err)
	}
	u.ID = StringOrInt(idToken.Subject)
	return
}
