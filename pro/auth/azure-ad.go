package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
	"gorm.io/gorm"
)

// AzureADProvider implements Provider for Azure Active Directory OAuth2.
type AzureADProvider struct {
	cfg *oauth2.Config
}

// NewAzureADProvider constructs an AzureADProvider for the given OAuth2 credentials.
func NewAzureADProvider(ctx context.Context, redirectURL, clientID, clientSecret string) *AzureADProvider {
	return &AzureADProvider{
		cfg: &oauth2.Config{
			RedirectURL:  redirectURL,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"User.Read", "email", "profile", "openid"},
			Endpoint:     microsoft.AzureADEndpoint(logic.GetAzureTenant(ctx)),
		},
	}
}

func (p *AzureADProvider) Name() string {
	return azure_ad_provider_name
}

func (p *AzureADProvider) Config() *oauth2.Config {
	return p.cfg
}

func (p *AzureADProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
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

func (p *AzureADProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var rState, rCode = getStateAndCode(r)
	state, err := logic.GetState(rState)
	if err != nil {
		handleOauthNotValid(w)
		return
	}

	content, err := p.GetUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from azure:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") || strings.Contains(err.Error(), "failed to fetch user email from SSO state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}

	var inviteExists bool
	in, err := logic.GetUserInvite(content.Email)
	if err == nil {
		inviteExists = true
	}
	if !inviteExists && (logic.IsPendingUser(content.Email) || logic.IsPendingUser(content.UserPrincipalName)) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}

	user, err := GetMatchingUser(content)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if inviteExists {
				user, err := proLogic.PrepareOauthUserFromInvite(in)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
					return
				}
				user.Username = content.UserPrincipalName
				user.ExternalIdentityProviderID = string(content.ID)
				err = orchestrator.GetRepository().UserOrchestrator().CreateUser(r.Context(), &user)
				if err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(content.Email)
				logic.DeletePendingUser(content.UserPrincipalName)
				logic.DeletePendingUser(content.Email)
			} else {
				if !isEmailAllowed(r.Context(), content.Email) {
					handleOauthUserNotAllowedToSignUp(w)
					return
				}
				pendingUser := &schema.PendingUser{
					TenantID:                   scope.ID(r.Context()),
					Username:                   content.UserPrincipalName,
					ExternalIdentityProviderID: string(content.ID),
				}
				err := pendingUser.Create(r.Context())
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

	user, err = GetMatchingUser(content)
	if err != nil {
		handleOauthUserNotFound(w)
		return
	}

	if user.AccountDisabled {
		handleUserAccountDisabled(w)
		return
	}

	userRole := &schema.UserRole{ID: user.PlatformRoleID}
	err = userRole.Get(r.Context())
	if err != nil {
		handleSomethingWentWrong(w)
		return
	}
	if userRole.DenyDashboardAccess {
		handleOauthUserNotAllowed(w)
		return
	}

	newPass, fetchErr := logic.FetchOAuthSecret()
	if fetchErr != nil {
		return
	}
	authRequest := models.UserAuthParams{
		UserName: content.UserPrincipalName,
		Password: newPass,
	}
	jwt, jwtErr := logic.VerifyAuthRequest(r.Context(), authRequest, state.AppName)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName)
		return
	}

	logic.LogEvent(&models.Event{
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
	logger.Log(1, "completed azure OAuth signin for", user.Username)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+user.Username, http.StatusPermanentRedirect)
}

func (p *AzureADProvider) GetUserInfo(state string, code string) (*OAuthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := p.cfg.Exchange(context.Background(), code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	data, err := json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token to json: %s", err.Error())
	}
	httpReq, reqErr := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request to microsoft")
	}
	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	userInfo := &OAuthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	if userInfo.Email == "" {
		userInfo.Email = getUserEmailFromClaims(token.AccessToken)
	}
	if userInfo.Email == "" && userInfo.UserPrincipalName != "" {
		userInfo.Email = userInfo.UserPrincipalName
	}
	if userInfo.Email == "" {
		return userInfo, errors.New("failed to fetch user email from SSO state")
	}
	return userInfo, nil
}

// GetMatchingUser looks up an Azure AD user by UPN or external provider ID.
func GetMatchingUser(oauthUser *OAuthUser) (*schema.User, error) {
	user := &schema.User{Username: oauthUser.UserPrincipalName}
	err := user.Get(db.WithContext(context.TODO()))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		return user, nil
	}

	user = &schema.User{ExternalIdentityProviderID: string(oauthUser.ID)}
	err = user.GetByExternalID(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}
	return user, nil
}
