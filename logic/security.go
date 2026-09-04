package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

const (
	MasterUser       = "masteradministrator"
	Forbidden_Msg    = "forbidden"
	Unauthorized_Msg = "unauthorized"
	Unauthorized_Err = models.Error(Unauthorized_Msg)
)

var NetworkPermissionsCheck = func(username string, r *http.Request) error { return nil }
var TenantPermissionsCheck = func(username string, r *http.Request) error { return nil }
var OrgPermissionsCheck = func(username string, r *http.Request) error { return nil }

// SecurityCheck - Check if user has appropriate permissions
func SecurityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("ismaster", "no")
		bearerToken := r.Header.Get("Authorization")
		username, err := GetUserNameFromToken(r.Context(), bearerToken)
		if err != nil {
			ReturnErrorResponse(w, r, FormatError(err, "unauthorized"))
			return
		}
		if username != MasterUser {
			user := &schema.User{Username: username}
			err = user.Get(r.Context())
			if err != nil {
				ReturnErrorResponse(w, r, FormatError(err, "unauthorized"))
				return
			}

			if user.AccountDisabled {
				err = errors.New("user account disabled")
				ReturnErrorResponse(w, r, FormatError(err, "unauthorized"))
				return
			}
		}

		// detect masteradmin
		if username == MasterUser {
			r.Header.Set("ismaster", "yes")
		} else {
			switch scope.Level(r.Context()) {
			case scope.OrgScope:
				err = OrgPermissionsCheck(username, r)
			case scope.TenantScope:
				if r.Header.Get("IS_NETWORK_ACCESS") == "yes" {
					err = NetworkPermissionsCheck(username, r)
				} else {
					err = TenantPermissionsCheck(username, r)
				}
				w.Header().Set("TARGET_RSRC", r.Header.Get("TARGET_RSRC"))
				w.Header().Set("TARGET_RSRC_ID", r.Header.Get("TARGET_RSRC_ID"))
				w.Header().Set("RSRC_TYPE", r.Header.Get("RSRC_TYPE"))
				w.Header().Set("IS_NETWORK_ACCESS", r.Header.Get("IS_NETWORK_ACCESS"))
				w.Header().Set("Access-Control-Allow-Origin", "*")
			default:
				err = nil
			}
		}
		if err != nil {
			w.Header().Set("ACCESS_PERM", err.Error())
			ReturnErrorResponse(w, r, FormatError(err, "forbidden"))
			return
		}
		r.Header.Set("user", username)
		next.ServeHTTP(w, r)
	}
}

func PreAuthCheck(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		headerSplits := strings.Split(authHeader, " ")
		if len(headerSplits) != 2 {
			ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
			return
		}

		authToken := headerSplits[1]

		// first check is user is authenticated.
		// if yes, allow the user to go through.
		username, err := GetUserNameFromToken(r.Context(), authHeader)
		if err != nil {
			// if no, then check the user has a pre-auth token.
			claims := &models.UserClaims{}
			token, err := jwt.ParseWithClaims(authToken, claims, func(token *jwt.Token) (interface{}, error) {
				return jwtSecretKey, nil
			})
			if err != nil {
				ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
				return
			}

			if token != nil && token.Valid {
				if len(claims.Audience) > 0 {
					var found bool
					for _, aud := range claims.Audience {
						if aud == "auth:mfa" {
							found = true
						}
					}

					if !found {
						ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
						return
					}

					username := claims.UserName
					if username == "" && claims.Scope == scope.GlobalScope && claims.ScopeID == "" {
						// token predates scope-aware claims; fall back to the
						// plain-username subject and assume it was issued for
						// the sole tenant that existed before multi-tenancy.
						username = claims.Subject

						soleTenant, err := SoleTenant(r.Context())
						if err != nil || soleTenant.ID != scope.ID(r.Context()) {
							ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
							return
						}
					} else if claims.Scope != scope.Level(r.Context()) || claims.ScopeID != scope.ID(r.Context()) {
						ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
						return
					}

					r.Header.Set("user", username)
					next.ServeHTTP(w, r)
					return
				} else {
					ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
					return
				}
			} else {
				ReturnErrorResponse(w, r, FormatError(Unauthorized_Err, "unauthorized"))
				return
			}
		} else {
			r.Header.Set("user", username)
			next.ServeHTTP(w, r)
			return
		}
	}
}

func ContinueIfUserMatch(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: Forbidden_Msg,
		}
		var params = mux.Vars(r)
		var requestedUser = params["username"]
		if requestedUser == "" {
			requestedUser = r.URL.Query().Get("username")
		}
		if requestedUser != r.Header.Get("user") {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func ContinueIfUserMatchOrAdmin(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: Forbidden_Msg,
		}

		username := r.Header.Get("user")
		if username == MasterUser {
			next.ServeHTTP(w, r)
			return
		}

		user := &schema.User{
			Username: username,
		}
		err := user.Get(r.Context())
		if err != nil {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}

		switch scope.Level(r.Context()) {
		case scope.OrgScope:
			next.ServeHTTP(w, r)
			return
		case scope.TenantScope:
			if user.PlatformRoleID == schema.SuperAdminRole || user.PlatformRoleID == schema.AdminRole {
				next.ServeHTTP(w, r)
				return
			}
		default:
			ReturnErrorResponse(w, r, errorResponse)
			return
		}

		var params = mux.Vars(r)
		var requestedUser = params["username"]
		if requestedUser == "" {
			requestedUser = r.URL.Query().Get("username")
		}
		if requestedUser != r.Header.Get("user") {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}
