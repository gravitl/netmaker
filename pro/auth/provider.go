package auth

import (
	"net/http"

	"golang.org/x/oauth2"
)

// Provider is implemented by each OAuth provider.
type Provider interface {
	Name() string
	Config() *oauth2.Config
	HandleLogin(http.ResponseWriter, *http.Request)
	HandleCallback(http.ResponseWriter, *http.Request)
	GetUserInfo(state, code string) (*OAuthUser, error)
}
