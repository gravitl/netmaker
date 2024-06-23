package logic

import (
	"errors"
	"fmt"
	"net/http"
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

func GetSubjectsFromURL(URL string) (rsrcType models.RsrcType, rsrcID models.RsrcID) {
	urlSplit := strings.Split(URL, "/")
	rsrcType = models.RsrcType(urlSplit[1])
	if len(urlSplit) > 1 {
		rsrcID = models.RsrcID(urlSplit[2])
	}
	return
}

func networkPermissionsCheck(username string, r *http.Request) error {
	// at this point global checks should be completed
	user, err := GetUser(username)
	if err != nil {
		return err
	}
	// get info from header to determine the target rsrc
	targetRsrc := r.Header.Get("TARGET_RSRC")
	targetRsrcID := r.Header.Get("TARGET_RSRC_ID")
	netID := r.Header.Get("NET_ID")
	if targetRsrc == "" {
		return errors.New("target rsrc is missing")
	}
	if netID == "" {
		return errors.New("network id is missing")
	}
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	// check if user has scope for target resource
	// TODO - differentitate between global scope and network scope apis
	networkPermissionScope, err := GetRole(user.NetworkRoles[models.NetworkID(netID)].String())
	if err != nil {
		return errors.New("access denied")
	}
	if networkPermissionScope.FullAccess {
		return nil
	}
	rsrcPermissionScope, ok := networkPermissionScope.NetworkLevelAccess[models.RsrcType(targetRsrc)]
	if !ok {
		return fmt.Errorf("access denied to %s rsrc", targetRsrc)
	}
	if allRsrcsTypePermissionScope, ok := rsrcPermissionScope[models.RsrcID(fmt.Sprintf("all_%s", targetRsrc))]; ok {
		return checkPermissionScopeWithReqMethod(allRsrcsTypePermissionScope, r.Method)

	}
	if targetRsrcID == "" {
		return errors.New("target rsrc is missing")
	}
	if scope, ok := rsrcPermissionScope[models.RsrcID(targetRsrcID)]; ok {
		return checkPermissionScopeWithReqMethod(scope, r.Method)
	}
	return errors.New("access denied")
}

func globalPermissionsCheck(username string, r *http.Request) error {
	user, err := GetUser(username)
	if err != nil {
		return err
	}
	userRole, err := GetRole(user.PlatformRoleID.String())
	if err != nil {
		return errors.New("access denied")
	}
	if userRole.FullAccess {
		return nil
	}
	targetRsrc := r.Header.Get("TARGET_RSRC")
	targetRsrcID := r.Header.Get("TARGET_RSRC_ID")
	if targetRsrc == "" {
		return errors.New("target rsrc is missing")
	}
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	rsrcPermissionScope, ok := userRole.GlobalLevelAccess[models.RsrcType(targetRsrc)]
	if !ok {
		return fmt.Errorf("access denied to %s rsrc", targetRsrc)
	}
	if allRsrcsTypePermissionScope, ok := rsrcPermissionScope[models.RsrcID(fmt.Sprintf("all_%s", targetRsrc))]; ok {
		return checkPermissionScopeWithReqMethod(allRsrcsTypePermissionScope, r.Method)

	}
	if targetRsrcID == "" {
		return errors.New("target rsrc id is missing")
	}
	if scope, ok := rsrcPermissionScope[models.RsrcID(targetRsrcID)]; ok {
		return checkPermissionScopeWithReqMethod(scope, r.Method)
	}
	return errors.New("access denied")
}

func checkPermissionScopeWithReqMethod(scope models.RsrcPermissionScope, reqmethod string) error {
	if reqmethod == http.MethodGet && scope.Read {
		return nil
	}
	if (reqmethod == http.MethodPatch || reqmethod == http.MethodPut) && scope.Update {
		return nil
	}
	if reqmethod == http.MethodDelete && scope.Delete {
		return nil
	}
	if reqmethod == http.MethodPost && scope.Create {
		return nil
	}
	return errors.New("operation not permitted")
}

// SecurityCheck - Check if user has appropriate permissions
func SecurityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		logger.Log(0, "SECURITY CHECK - 1")
		r.Header.Set("ismaster", "no")
		bearerToken := r.Header.Get("Authorization")
		isGlobalAccesss := r.Header.Get("IS_GLOBAL_ACCESS") == "yes"
		username, err := UserPermissions(reqAdmin, bearerToken)
		if err != nil {
			logger.Log(0, "SECURITY CHECK - 2", err.Error())
			ReturnErrorResponse(w, r, FormatError(err, err.Error()))
			return
		}
		// detect masteradmin
		if username == MasterUser {
			r.Header.Set("ismaster", "yes")
		} else {
			if isGlobalAccesss {
				err = globalPermissionsCheck(username, r)
			} else {
				err = networkPermissionsCheck(username, r)
			}
		}
		w.Header().Set("TARGET_RSRC", r.Header.Get("TARGET_RSRC"))
		w.Header().Set("TARGET_RSRC_ID", r.Header.Get("TARGET_RSRC_ID"))
		w.Header().Set("RSRC_TYPE", r.Header.Get("RSRC_TYPE"))
		w.Header().Set("IS_GLOBAL_ACCESS", r.Header.Get("IS_GLOBAL_ACCESS"))
		w.Header().Set("ACCESS_PERM", err.Error())
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
		if requestedUser != r.Header.Get("user") {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}
