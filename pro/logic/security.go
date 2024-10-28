package logic

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// constants for accounts api hosts
const (
	// accountsHostDevelopment is the accounts api host for development environment
	accountsHostDevelopment = "https://api.dev.accounts.netmaker.io"
	// accountsHostStaging is the accounts api host for staging environment
	accountsHostStaging = "https://api.staging.accounts.netmaker.io"
	// accountsHostProduction is the accounts api host for production environment
	accountsHostProduction = "https://api.accounts.netmaker.io"
)

// constants for accounts UI hosts
const (
	// accountsUIHostDevelopment is the accounts UI host for development environment
	accountsUIHostDevelopment = "https://account.dev.netmaker.io"
	// accountsUIHostStaging is the accounts UI host for staging environment
	accountsUIHostStaging = "https://account.staging.netmaker.io"
	// accountsUIHostProduction is the accounts UI host for production environment
	accountsUIHostProduction = "https://account.netmaker.io"
)

func NetworkPermissionsCheck(username string, r *http.Request) error {
	// at this point global checks should be completed
	user, err := logic.GetUser(username)
	if err != nil {
		return err
	}
	userRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		return errors.New("access denied")
	}
	if userRole.FullAccess {
		return nil
	}
	// get info from header to determine the target rsrc
	targetRsrc := r.Header.Get("TARGET_RSRC")
	targetRsrcID := r.Header.Get("TARGET_RSRC_ID")
	netID := r.Header.Get("NET_ID")
	if targetRsrc == "" {
		return errors.New("target rsrc is missing")
	}
	if r.Header.Get("RAC") == "true" && r.Method == http.MethodGet {
		return nil
	}
	if netID == "" {
		return errors.New("network id is missing")
	}
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	if targetRsrc == models.MetricRsrc.String() {
		return nil
	}

	// check if user has scope for target resource
	// TODO - differentitate between global scope and network scope apis
	// check for global network role

	for groupID := range user.UserGroups {
		userG, err := GetUserGroup(groupID)
		if err == nil {
			netRoles := userG.NetworkRoles[models.NetworkID(netID)]
			for netRoleID := range netRoles {
				err = checkNetworkAccessPermissions(netRoleID, username, r.Method, targetRsrc, targetRsrcID, netID)
				if err == nil {
					return nil
				}
			}
		}
	}

	return errors.New("access denied")
}

func checkNetworkAccessPermissions(netRoleID models.UserRoleID, username, reqScope, targetRsrc, targetRsrcID, netID string) error {
	networkPermissionScope, err := logic.GetRole(netRoleID)
	if err != nil {
		return err
	}
	if networkPermissionScope.FullAccess {
		return nil
	}
	rsrcPermissionScope, ok := networkPermissionScope.NetworkLevelAccess[models.RsrcType(targetRsrc)]
	if targetRsrc == models.HostRsrc.String() && !ok {
		rsrcPermissionScope, ok = networkPermissionScope.NetworkLevelAccess[models.RemoteAccessGwRsrc]
	}
	if !ok {
		return errors.New("access denied")
	}
	if allRsrcsTypePermissionScope, ok := rsrcPermissionScope[models.RsrcID(fmt.Sprintf("all_%s", targetRsrc))]; ok {
		// handle extclient apis here
		if models.RsrcType(targetRsrc) == models.ExtClientsRsrc && allRsrcsTypePermissionScope.SelfOnly && targetRsrcID != "" {
			extclient, err := logic.GetExtClient(targetRsrcID, netID)
			if err != nil {
				return err
			}
			if !logic.IsUserAllowedAccessToExtClient(username, extclient) {
				return errors.New("access denied")
			}
		}
		err = checkPermissionScopeWithReqMethod(allRsrcsTypePermissionScope, reqScope)
		if err == nil {
			return nil
		}

	}
	if targetRsrc == models.HostRsrc.String() {
		if allRsrcsTypePermissionScope, ok := rsrcPermissionScope[models.RsrcID(fmt.Sprintf("all_%s", models.RemoteAccessGwRsrc))]; ok {
			err = checkPermissionScopeWithReqMethod(allRsrcsTypePermissionScope, reqScope)
			if err == nil {
				return nil
			}
		}
	}
	if targetRsrcID == "" {
		return errors.New("target rsrc id is empty")
	}
	if scope, ok := rsrcPermissionScope[models.RsrcID(targetRsrcID)]; ok {
		err = checkPermissionScopeWithReqMethod(scope, reqScope)
		if err == nil {
			return nil
		}
	}
	return errors.New("access denied")
}

func GlobalPermissionsCheck(username string, r *http.Request) error {
	user, err := logic.GetUser(username)
	if err != nil {
		return err
	}
	userRole, err := logic.GetRole(user.PlatformRoleID)
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
	if targetRsrc == models.MetricRsrc.String() {
		return nil
	}
	if (targetRsrc == models.HostRsrc.String() || targetRsrc == models.NetworkRsrc.String()) && r.Method == http.MethodGet && targetRsrcID == "" {
		return nil
	}
	if targetRsrc == models.UserRsrc.String() && username == targetRsrcID && (r.Method != http.MethodDelete) {
		return nil
	}
	rsrcPermissionScope, ok := userRole.GlobalLevelAccess[models.RsrcType(targetRsrc)]
	if !ok {
		return fmt.Errorf("access denied to %s", targetRsrc)
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

func GetAccountsHost() string {
	switch servercfg.GetEnvironment() {
	case "dev":
		return accountsHostDevelopment
	case "staging":
		return accountsHostStaging
	default:
		return accountsHostProduction
	}
}

func GetAccountsUIHost() string {
	switch servercfg.GetEnvironment() {
	case "dev":
		return accountsUIHostDevelopment
	case "staging":
		return accountsUIHostStaging
	default:
		return accountsUIHostProduction
	}
}
