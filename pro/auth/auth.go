package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
)

// == consts ==
const (
	google_provider_name   = "google"
	azure_ad_provider_name = "azure-ad"
	github_provider_name   = "github"
	okta_provider_name     = "okta"
	oidc_provider_name     = "oidc"
	user_signin_length     = 16
	node_signin_length     = 64
	headless_signin_length = 32
)

// OAuthUser - generic OAuth strategy user
type OAuthUser struct {
	ID                StringOrInt `json:"id" bson:"id"`
	Name              string      `json:"name" bson:"name"`
	Email             string      `json:"email" bson:"email"`
	Login             string      `json:"login" bson:"login"`
	UserPrincipalName string      `json:"userPrincipalName" bson:"userPrincipalName"`
	AccessToken       string      `json:"accesstoken" bson:"accesstoken"`
}

// TODO: this is a very poor solution.
// We should not return the same OAuthUser for different
// IdPs. They should have the user that their APIs return.
// But that's a very big change. So, making do with this
// for now.

type StringOrInt string

func (s *StringOrInt) UnmarshalJSON(data []byte) error {
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		*s = StringOrInt(strVal)
		return nil
	}

	var intVal int
	if err := json.Unmarshal(data, &intVal); err == nil {
		*s = StringOrInt(strconv.Itoa(intVal))
		return nil
	}

	return fmt.Errorf("cannot unmarshal %s into StringOrInt", string(data))
}

var upgrader = websocket.Upgrader{}

// ResetAuthProvider resets the auth provider for the scope in ctx.
func ResetAuthProvider(ctx context.Context) {
	Registry().Delete(scope.Level(ctx), scope.ID(ctx))
}

// IsOAuthConfigured reports whether an OAuth provider is configured for the scope in ctx.
func IsOAuthConfigured(ctx context.Context) bool {
	_, ok := Registry().FromContext(ctx)
	return ok
}

// HandleAuthCallback handles oauth callback.
// Note: not included in API reference as part of the OAuth process itself.
func HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	state, _ := getStateAndCode(r)

	var isHeadlessLogin bool
	cValue, err := netcache.Get(state)
	if err != nil {
		if errors.Is(err, netcache.ErrExpired) {
			isHeadlessLogin = true
		}
	} else {
		isHeadlessLogin = true
	}

	if isHeadlessLogin {
		if cValue != nil {
			ctx := scope.WithContext(r.Context(), cValue.Scope, cValue.ScopeID)
			_, ok := Registry().FromContext(ctx)
			if !ok {
				handleOauthNotConfigured(w)
				return
			}

			r = r.WithContext(ctx)
		}
		switch len(state) {
		case node_signin_length:
			logger.Log(1, "proceeding with host SSO callback")
			HandleHostSSOCallback(w, r)
		case headless_signin_length:
			logger.Log(1, "proceeding with headless SSO callback")
			HandleHeadlessSSOCallback(w, r)
		default:
			logger.Log(1, "invalid state length: ", fmt.Sprintf("%d", len(state)))
			handleSomethingWentWrong(w)
		}
	} else {
		ssoState, err := logic.GetState(state)
		if err != nil {
			handleSomethingWentWrong(w)
			return
		}

		if ssoState.Scope == 0 || ssoState.ScopeID == "" {
			handleSomethingWentWrong(w)
			return
		}

		ctx := scope.WithContext(r.Context(), ssoState.Scope, ssoState.ScopeID)
		provider, ok := Registry().FromContext(ctx)
		if !ok {
			handleOauthNotConfigured(w)
			return
		}

		provider.HandleCallback(w, r.WithContext(ctx))
	}
}

// swagger:route GET /api/oauth/login nodes HandleAuthLogin
//
// Handles OAuth login.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//			Responses:
//			200:  okResponse
func HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	provider, ok := Registry().FromContext(r.Context())
	if !ok {
		handleOauthNotConfigured(w)
		return
	}
	if servercfg.GetFrontendURL() == "" {
		handleOauthNotConfigured(w)
		return
	}
	if r.Header.Get("X-Application-Name") == "" && r.URL.Query().Has("x-application-name") {
		r.Header.Set("X-Application-Name", r.URL.Query().Get("x-application-name"))
	}

	provider.HandleLogin(w, r)
}

// HandleHeadlessSSO handles the OAuth login flow for headless interfaces such as Netmaker CLI via websocket.
func HandleHeadlessSSO(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log(0, "error during connection upgrade for headless sign-in:", err.Error())
		return
	}
	if conn == nil {
		logger.Log(0, "failed to establish web-socket connection during headless sign-in")
		return
	}
	defer conn.Close()

	req := &netcache.CValue{
		Scope:   scope.Level(r.Context()),
		ScopeID: scope.ID(r.Context()),
	}
	stateStr := logic.RandomString(headless_signin_length)
	if err = netcache.Set(stateStr, req); err != nil {
		logger.Log(0, "Failed to process sso request -", err.Error())
		return
	}

	timeout := make(chan bool, 1)
	answer := make(chan string, 1)
	defer close(answer)
	defer close(timeout)

	if !IsOAuthConfigured(r.Context()) {
		if err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			logger.Log(0, "error during message writing:", err.Error())
		}
		return
	}
	redirectUrl = fmt.Sprintf("https://%s/api/oauth/register/%s", servercfg.GetAPIConnString(), stateStr)
	if err = conn.WriteMessage(websocket.TextMessage, []byte(redirectUrl)); err != nil {
		logger.Log(0, "error during message writing:", err.Error())
	}

	go func() {
		for {
			cachedReq, err := netcache.Get(stateStr)
			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					logger.Log(0, "timeout occurred while waiting for SSO")
					timeout <- true
					break
				}
				continue
			} else if cachedReq.Pass != "" {
				logger.Log(0, "SSO process completed for user ", cachedReq.User)
				answer <- cachedReq.Pass
				break
			}
			time.Sleep(500) // try it 2 times per second to see if auth is completed
		}
	}()

	select {
	case result := <-answer:
		if err = conn.WriteMessage(websocket.TextMessage, []byte(result)); err != nil {
			logger.Log(0, "Error during message writing:", err.Error())
		}
	case <-timeout:
		logger.Log(0, "Authentication server time out for headless SSO login")
		if err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			logger.Log(0, "Error during message writing:", err.Error())
		}
	}
	if err = netcache.Del(stateStr); err != nil {
		logger.Log(0, "failed to remove SSO cache entry", err.Error())
	}
	if err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		logger.Log(0, "write close:", err.Error())
	}
}

// == private helpers ==

func getStateAndCode(r *http.Request) (string, string) {
	var state, code string
	if r.FormValue("state") != "" && r.FormValue("code") != "" {
		state = r.FormValue("state")
		code = r.FormValue("code")
	} else if r.URL.Query().Get("state") != "" && r.URL.Query().Get("code") != "" {
		state = r.URL.Query().Get("state")
		code = r.URL.Query().Get("code")
	}
	return state, code
}

func getUserEmailFromClaims(token string) string {
	accessToken, _ := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(""), nil
	})
	if accessToken == nil {
		return ""
	}
	claims, _ := accessToken.Claims.(jwt.MapClaims)
	if claims == nil {
		return ""
	}
	if claims["email"] == nil {
		return ""
	}
	return claims["email"].(string)
}

func (user *OAuthUser) getUserName() string {
	var userName string
	if user.Email != "" {
		userName = user.Email
	} else if user.Login != "" {
		userName = user.Login
	} else if user.UserPrincipalName != "" {
		userName = user.UserPrincipalName
	} else if user.Name != "" {
		userName = user.Name
	}
	return userName
}

func isStateCached(state string) bool {
	_, err := netcache.Get(state)
	return err == nil || strings.Contains(err.Error(), "expired")
}

// isEmailAllowed checks if email is allowed to signup.
func isEmailAllowed(ctx context.Context, email string) bool {
	allowedDomains := logic.GetAllowedEmailDomains(ctx)
	domains := strings.Split(allowedDomains, ",")
	if len(domains) == 1 && domains[0] == "*" {
		return true
	}
	emailParts := strings.Split(email, "@")
	if len(emailParts) < 2 {
		return false
	}
	baseDomainOfEmail := emailParts[1]
	for _, domain := range domains {
		if domain == baseDomainOfEmail {
			return true
		}
	}
	return false
}
