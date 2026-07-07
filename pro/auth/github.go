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
	"golang.org/x/oauth2/github"
	"gorm.io/gorm"
)

// GitHubProvider implements Provider for GitHub OAuth2.
type GitHubProvider struct {
	cfg *oauth2.Config
}

// NewGitHubProvider constructs a GitHubProvider for the given OAuth2 credentials.
func NewGitHubProvider(redirectURL, clientID, clientSecret string) *GitHubProvider {
	return &GitHubProvider{
		cfg: &oauth2.Config{
			RedirectURL:  redirectURL,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		},
	}
}

func (p *GitHubProvider) Name() string {
	return github_provider_name
}

func (p *GitHubProvider) Config() *oauth2.Config {
	return p.cfg
}

func (p *GitHubProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
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

func (p *GitHubProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var rState, rCode = getStateAndCode(r)

	state, err := logic.GetState(rState)
	if err != nil {
		handleOauthNotValid(w)
		return
	}

	content, err := p.GetUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from github:", err.Error())
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
	if !inviteExists && logic.IsPendingUser(content.Email) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}

	// if user exists with provider login ID, migrate them to email-based username
	user := &schema.User{Username: content.Login}
	err = user.Get(r.Context())
	if err == nil {
		if user.AuthType == schema.BasicAuth {
			logger.Log(0, "invalid auth type: basic_auth")
			handleAuthTypeMismatch(w)
			return
		}
		emailCheck := &schema.User{Username: content.Email}
		err = emailCheck.Get(r.Context())
		if err != nil {
			user.Username = content.Email
			user.ExternalIdentityProviderID = content.Login
			_ = user.Update(db.WithContext(context.TODO()))
		}
	}

	emailCheck := &schema.User{Username: content.Email}
	err = emailCheck.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if inviteExists {
				user, err := proLogic.PrepareOauthUserFromInvite(in)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
					return
				}
				user.ExternalIdentityProviderID = string(content.ID)
				err = orchestrator.GetRepository().UserOrchestrator().CreateUser(r.Context(), &user)
				if err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(content.Email)
				logic.DeletePendingUser(content.Email)
			} else {
				if !isEmailAllowed(content.Email) {
					handleOauthUserNotAllowedToSignUp(w)
					return
				}
				pendingUser := &schema.PendingUser{
					Username:                   content.Email,
					ExternalIdentityProviderID: string(content.ID),
				}
				if pendingUser.TenantID == "" {
					pendingUser.TenantID = scope.ID(logic.DefaultScope(r.Context()))
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
	}

	user = &schema.User{Username: content.Email}
	err = user.Get(r.Context())
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
		UserName: content.Email,
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
	logger.Log(1, "completed github OAuth signin for", content.Email)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.Email, http.StatusPermanentRedirect)
}

func (p *GitHubProvider) GetUserInfo(state, code string) (*OAuthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := p.cfg.Exchange(context.Background(), code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	if !token.Valid() {
		return nil, fmt.Errorf("GitHub code exchange yielded invalid token")
	}
	data, err := json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token to json: %s", err.Error())
	}
	httpReq, reqErr := http.NewRequest("GET", "https://api.github.com/user", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request to GitHub")
	}
	httpReq.Header.Set("Authorization", "token "+token.AccessToken)
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
		logger.Log(2, "fetching user email from github api")
		userInfo.Email, err = getGithubEmailsInfo(token.AccessToken)
		if err != nil {
			logger.Log(0, "failed to fetch user's email from github: ", err.Error())
		}
	}
	if userInfo.Email == "" {
		return userInfo, errors.New("failed to fetch user email from SSO state")
	}
	return userInfo, nil
}

func getGithubEmailsInfo(accessToken string) (string, error) {
	httpReq, reqErr := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if reqErr != nil {
		return "", fmt.Errorf("failed to create request to GitHub")
	}
	httpReq.Header.Add("Accept", "application/vnd.github.v3+json")
	httpReq.Header.Set("Authorization", "token "+accessToken)
	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed reading response body: %s", err.Error())
	}
	var emailsInfo []interface{}
	if err = json.Unmarshal(contents, &emailsInfo); err != nil {
		return "", err
	}
	for _, info := range emailsInfo {
		emailInfoMap := info.(map[string]interface{})
		if emailInfoMap["primary"].(bool) {
			return emailInfoMap["email"].(string), nil
		}
	}
	return "", errors.New("email not found")
}
