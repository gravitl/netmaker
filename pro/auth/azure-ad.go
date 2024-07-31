package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

var azure_ad_functions = map[string]interface{}{
	init_provider:   initAzureAD,
	get_user_info:   getAzureUserInfo,
	handle_callback: handleAzureCallback,
	handle_login:    handleAzureLogin,
	verify_user:     verifyAzureUser,
}

// == handle azure ad authentication here ==

func initAzureAD(redirectURL string, clientID string, clientSecret string) {
	auth_provider = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"User.Read"},
		Endpoint:     microsoft.AzureADEndpoint(servercfg.GetAzureTenant()),
	}
}

func handleAzureLogin(w http.ResponseWriter, r *http.Request) {
	var oauth_state_string = logic.RandomString(user_signin_length)
	if auth_provider == nil {
		handleOauthNotConfigured(w)
		return
	}

	if err := logic.SetState(oauth_state_string); err != nil {
		handleOauthNotConfigured(w)
		return
	}

	var url = auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleAzureCallback(w http.ResponseWriter, r *http.Request) {

	var rState, rCode = getStateAndCode(r)
	var content, err = getAzureUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from azure:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}
	if !isEmailAllowed(content.UserPrincipalName) {
		handleOauthUserNotAllowedToSignUp(w)
		return
	}
	var inviteExists bool
	// check if invite exists for User
	in, err := logic.GetUserInvite(content.UserPrincipalName)
	if err == nil {
		inviteExists = true
	}
	// check if user approval is already pending
	if !inviteExists && logic.IsPendingUser(content.UserPrincipalName) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}

	_, err = logic.GetUser(content.UserPrincipalName)
	if err != nil {
		if database.IsEmptyRecord(err) { // user must not exist, so try to make one
			if inviteExists {
				// create user
				var newPass, fetchErr = logic.FetchPassValue("")
				if fetchErr != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(fetchErr, "internal"))
					return
				}
				user := &models.User{
					UserName: content.UserPrincipalName,
					Password: newPass,
				}
				for _, inviteGroupID := range in.Groups {
					_, err := proLogic.GetUserGroup(inviteGroupID)
					if err != nil {
						logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("error fetching group id "+inviteGroupID.String()), "badrequest"))
						return
					}

					user.UserGroups[inviteGroupID] = struct{}{}
				}
				user.PlatformRoleID = models.UserRole(in.PlatformRoleID)
				if user.PlatformRoleID == "" {
					user.PlatformRoleID = models.ServiceUser
				}
				if err = logic.CreateUser(user); err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(user.UserName)
				logic.DeletePendingUser(content.UserPrincipalName)
			} else {
				err = logic.InsertPendingUser(&models.User{
					UserName: content.UserPrincipalName,
				})
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
	user, err := logic.GetUser(content.UserPrincipalName)
	if err != nil {
		handleOauthUserNotFound(w)
		return
	}
	userRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		handleSomethingWentWrong(w)
		return
	}
	if userRole.DenyDashboardAccess {
		handleOauthUserNotAllowed(w)
		return
	}
	var newPass, fetchErr = logic.FetchPassValue("")
	if fetchErr != nil {
		return
	}
	// send a netmaker jwt token
	var authRequest = models.UserAuthParams{
		UserName: content.UserPrincipalName,
		Password: newPass,
	}

	var jwt, jwtErr = logic.VerifyAuthRequest(authRequest)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName)
		return
	}

	logger.Log(1, "completed azure OAuth sigin in for", content.UserPrincipalName)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.UserPrincipalName, http.StatusPermanentRedirect)
}

func getAzureUserInfo(state string, code string) (*OAuthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	var token, err = auth_provider.Exchange(context.Background(), code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	var data []byte
	data, err = json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token to json: %s", err.Error())
	}
	var httpReq, reqErr = http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request to GitHub")
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
	var userInfo = &OAuthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	return userInfo, nil
}

func verifyAzureUser(token *oauth2.Token) bool {
	return token.Valid()
}
