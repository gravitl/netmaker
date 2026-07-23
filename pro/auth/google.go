package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// GoogleProvider implements Provider for Google OAuth2.
type GoogleProvider struct {
	cfg *oauth2.Config
}

// NewGoogleProvider constructs a GoogleProvider for the given OAuth2 credentials.
func NewGoogleProvider(redirectURL, clientID, clientSecret string) *GoogleProvider {
	return &GoogleProvider{
		cfg: &oauth2.Config{
			RedirectURL:  redirectURL,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (p *GoogleProvider) Name() string {
	return google_provider_name
}

func (p *GoogleProvider) Config() *oauth2.Config {
	return p.cfg
}

func (p *GoogleProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
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

func (p *GoogleProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var rState, rCode = getStateAndCode(r)
	logger.Log(0, "Fetched OAuth State ", rState)

	state, err := logic.GetState(rState)
	if err != nil {
		handleOauthNotValid(w)
		return
	}

	content, err := p.GetUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from google:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") {
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

	user := &schema.User{Username: content.Email}
	err = user.Get(r.Context())
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
					if errors.Is(err, logic.ErrUserLimitExceeded) {
						handleUserLimitExceeded(w)
						return
					}
					handleSomethingWentWrong(w)
					return
				}

				logic.DeleteUserInvite(user.Username)

				_ = (&schema.PendingUser{
					Username: content.Email,
				}).Delete(r.Context())
			} else {
				if !isEmailAllowed(r.Context(), content.Email) {
					handleOauthUserNotAllowedToSignUp(w)
					return
				}
				pendingUser := &schema.PendingUser{
					TenantID:                   scope.ID(r.Context()),
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
	err = user.Get(r.Context())
	if err != nil {
		logger.Log(0, "error fetching user: ", err.Error())
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

	logger.Log(1, "completed google OAuth signin for", content.Email)
	http.Redirect(w, r, fmt.Sprintf("%s/login?login=%s&user=%s", servercfg.GetFrontendURL(), jwt, content.Email), http.StatusPermanentRedirect)
}

func (p *GoogleProvider) GetUserInfo(state string, code string) (*OAuthUser, error) {
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
	client := &http.Client{Timeout: time.Second * 30}
	response, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
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
	return userInfo, nil
}
