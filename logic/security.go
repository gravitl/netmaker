package logic

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	MasterUser       = "masteradministrator"
	Forbidden_Msg    = "forbidden"
	Forbidden_Err    = models.Error(Forbidden_Msg)
	Unauthorized_Msg = "unauthorized"
	Unauthorized_Err = models.Error(Unauthorized_Msg)
)

var NetworkPermissionsCheck = func(username string, r *http.Request) error { return nil }
var GlobalPermissionsCheck = func(username string, r *http.Request) error { return nil }

// SecurityCheck - Check if user has appropriate permissions
func SecurityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("ismaster", "no")
		logger.Log(0, "next", r.URL.String())
		isGlobalAccesss := r.Header.Get("IS_GLOBAL_ACCESS") == "yes"
		bearerToken := r.Header.Get("Authorization")
		username, err := GetUserNameFromToken(bearerToken)
		if err != nil {
			ReturnErrorResponse(w, r, FormatError(err, "unauthorized"))
			return
		}
		// detect masteradmin
		if username == MasterUser {
			r.Header.Set("ismaster", "yes")
		} else {
			if isGlobalAccesss {
				err = GlobalPermissionsCheck(username, r)
			} else {
				err = NetworkPermissionsCheck(username, r)
			}
		}
		w.Header().Set("TARGET_RSRC", r.Header.Get("TARGET_RSRC"))
		w.Header().Set("TARGET_RSRC_ID", r.Header.Get("TARGET_RSRC_ID"))
		w.Header().Set("RSRC_TYPE", r.Header.Get("RSRC_TYPE"))
		w.Header().Set("IS_GLOBAL_ACCESS", r.Header.Get("IS_GLOBAL_ACCESS"))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err != nil {
			w.Header().Set("ACCESS_PERM", err.Error())
			ReturnErrorResponse(w, r, FormatError(err, "forbidden"))
			return
		}
		r.Header.Set("user", username)
		next.ServeHTTP(w, r)
	}
}

// UserPermissions - checks token stuff
func UserPermissions(reqAdmin bool, token string) (string, error) {
	var tokenSplit = strings.Split(token, " ")
	var authToken = ""

	if len(tokenSplit) < 2 {
		return "", Unauthorized_Err
	} else {
		authToken = tokenSplit[1]
	}
	//all endpoints here require master so not as complicated
	if authenticateMaster(authToken) {
		// TODO log in as an actual admin user
		return MasterUser, nil
	}
	username, issuperadmin, isadmin, err := VerifyUserToken(authToken)
	if err != nil {
		return username, Unauthorized_Err
	}
	if reqAdmin && !(issuperadmin || isadmin) {
		return username, Forbidden_Err
	}

	return username, nil
}

// Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
	return tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != ""
}

func ContinueIfUserMatch(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: Forbidden_Msg,
		}
		var params = mux.Vars(r)
		var requestedUser = params["username"]
		if requestedUser == "" {
			requestedUser, _ = url.QueryUnescape(r.URL.Query().Get("username"))
		}
		if requestedUser != r.Header.Get("user") {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}
