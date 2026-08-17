package logic

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"context"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
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

	// saasNMUIHostDevelopment is the SaaS NMUI host for development environment
	saasNMUIHostDevelopment = "https://app.dev.netmaker.io"
	// saasNMUIHostStaging is the SaaS NMUI host for staging environment
	saasNMUIHostStaging = "https://app.staging.netmaker.io"
	// saasNMUIHostProduction is the SaaS NMUI host for production environment
	saasNMUIHostProduction = "https://app.netmaker.io"
)

func isDeviceAPIRequest(r *http.Request) bool {
	// Exact prefix match so paths like /api/v1/devices/... are not treated as device APIs.
	return strings.HasPrefix(r.URL.Path, "/api/v1/device/")
}

func NetworkPermissionsCheck(username string, r *http.Request) error {
	// Device APIs skip TARGET_RSRC middleware checks (desktop clients do not send
	// dashboard resource headers). Mutating device ops enforce write scopes in
	// UserHasDeviceNetworkWriteAccess instead.
	if isDeviceAPIRequest(r) {
		return nil
	}
	// at this point global checks should be completed
	user := &schema.User{Username: username}
	err := user.GetWithMembership(r.Context())
	if err != nil {
		return err
	}
	userRole := &schema.UserRole{ID: user.PlatformRoleID}
	err = userRole.GetPlatformRole(r.Context())
	if err != nil {
		return errors.New("access denied")
	}
	// Platform admin/super-admin FullAccess applies to global APIs only; network
	// APIs always require group-based network roles.
	if userRole.TenantGlobalAccess && !PlatformRoleRequiresGroupEnforcement(user.PlatformRoleID) {
		return nil
	}

	if userRole.ID == schema.Auditor {
		if r.Method == http.MethodGet {
			return nil
		} else {
			return errors.New("access denied")
		}
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

	for groupID := range user.UserGroups.Data() {

		userG, err := GetUserGroup(r.Context(), groupID)
		if err == nil {
			if netRoles, ok := userG.NetworkRoles.Data()[schema.AllNetworks]; ok {
				for netRoleID := range netRoles {
					err = checkNetworkAccessPermissions(r.Context(), netRoleID, username, r.Method, targetRsrc, targetRsrcID, netID)
					if err == nil {
						return nil
					}
				}
			}
			netRoles := userG.NetworkRoles.Data()[schema.NetworkID(netID)]
			for netRoleID := range netRoles {
				err = checkNetworkAccessPermissions(r.Context(), netRoleID, username, r.Method, targetRsrc, targetRsrcID, netID)
				if err == nil {
					return nil
				}
			}
		}
	}

	return errors.New("access denied")
}

func checkNetworkAccessPermissions(ctx context.Context, netRoleID schema.UserRoleID, username, reqScope, targetRsrc, targetRsrcID, netID string) error {
	networkPermissionScope := &schema.UserRole{ID: netRoleID}
	err := networkPermissionScope.GetNetworkRole(ctx)
	if err != nil {
		return err
	}
	if networkPermissionScope.TenantGlobalAccess {
		return nil
	}
	if networkPermissionScope.NetworkID.String() != netID {
		return errors.New("access denied")
	}
	rsrcPermissionScope, ok := networkPermissionScope.NetworkLevelAccess.Data()[schema.RsrcType(targetRsrc)]
	if !ok {
		return errors.New("access denied")
	}
	if allRsrcsTypePermissionScope, ok := rsrcPermissionScope[logic.GetAllRsrcIDForRsrc(schema.RsrcType(targetRsrc))]; ok {
		// handle extclient apis here
		if schema.RsrcType(targetRsrc) == schema.ExtClientsRsrc && allRsrcsTypePermissionScope.SelfOnly && targetRsrcID != "" {
			extclient, err := logic.GetExtClient(ctx, targetRsrcID, netID)
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
	if targetRsrcID == "" {
		return errors.New("target rsrc id is empty")
	}
	if scope, ok := rsrcPermissionScope[schema.RsrcID(targetRsrcID)]; ok {
		err = checkPermissionScopeWithReqMethod(scope, reqScope)
		if err == nil {
			return nil
		}
	}
	return errors.New("access denied")
}

func OrgPermissionsCheck(username string, r *http.Request) error {
	user := &schema.User{Username: username}
	err := user.GetWithMembership(r.Context())
	if err != nil {
		return err
	}
	if user.PlatformRoleID == schema.OrgOwner || user.PlatformRoleID == schema.OrgAdmin {
		return nil
	}
	return errors.New("access denied")
}

func TenantPermissionsCheck(username string, r *http.Request) error {
	// Same rationale as NetworkPermissionsCheck: device handlers enforce their own RBAC.
	if isDeviceAPIRequest(r) {
		return nil
	}
	route, err := mux.CurrentRoute(r).GetPathTemplate()
	if err != nil {
		return err
	}
	user := &schema.User{Username: username}
	err = user.GetWithMembership(r.Context())
	if err != nil {
		return err
	}
	userRole := &schema.UserRole{ID: user.PlatformRoleID}
	err = userRole.GetPlatformRole(r.Context())
	if err != nil {
		return errors.New("access denied")
	}
	if userRole.TenantGlobalAccess {
		return nil
	}
	if strings.Contains(r.URL.Path, "/api/v1/egress/presets") {
		return nil
	}
	if userRole.ID == schema.Auditor {
		if strings.Contains(r.URL.Path, "/api/v1/enrollment-keys") {
			return errors.New("access denied")
		}
		if r.Method == http.MethodGet {
			return nil
		} else {
			if (r.Method == http.MethodPut || r.Method == http.MethodPost) &&
				strings.Contains(r.URL.Path, "/api/users/"+username) {
				return nil
			}

			return errors.New("access denied")
		}
	}

	targetRsrc := r.Header.Get("TARGET_RSRC")
	targetRsrcID := r.Header.Get("TARGET_RSRC_ID")
	if targetRsrc == "" {
		return errors.New("target rsrc is missing")
	}
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	if targetRsrc == schema.MetricRsrc.String() {
		return nil
	}
	if (targetRsrc == schema.HostRsrc.String() || targetRsrc == schema.NetworkRsrc.String()) && r.Method == http.MethodGet && targetRsrcID == "" {
		return nil
	}
	if targetRsrc == schema.UserRsrc.String() && user.PlatformRoleID == schema.PlatformUser && r.Method == http.MethodPut &&
		route == "/api/v1/users/add_network_user" || route == "/api/v1/users/remove_network_user" {
		return nil
	}
	if targetRsrc == schema.UserRsrc.String() && user.PlatformRoleID == schema.PlatformUser && r.Method == http.MethodGet &&
		route == "/api/v1/users/unassigned_network_users" {
		return nil
	}
	if targetRsrc == schema.JitUserRsrc.String() && r.Method == http.MethodGet &&
		strings.Contains(r.URL.Path, "/api/v1/jit_user/networks") {
		return nil
	}
	if targetRsrc == schema.UserRsrc.String() && username == targetRsrcID && (r.Method != http.MethodDelete) {
		return nil
	}
	if r.Method == http.MethodGet && targetRsrc == schema.UserActivityRsrc.String() && route == "/api/v1/user/activity" {
		return nil
	}
	rsrcPermissionScope, ok := userRole.GlobalLevelAccess.Data()[schema.RsrcType(targetRsrc)]
	if !ok {
		return fmt.Errorf("access denied to %s", targetRsrc)
	}
	if allRsrcsTypePermissionScope, ok := rsrcPermissionScope[schema.RsrcID(fmt.Sprintf("all_%s", targetRsrc))]; ok {
		return checkPermissionScopeWithReqMethod(allRsrcsTypePermissionScope, r.Method)

	}
	if targetRsrcID == "" {
		return errors.New("target rsrc id is missing")
	}
	if scope, ok := rsrcPermissionScope[schema.RsrcID(targetRsrcID)]; ok {
		return checkPermissionScopeWithReqMethod(scope, r.Method)
	}
	return errors.New("access denied")
}

func checkPermissionScopeWithReqMethod(scope schema.RsrcPermissionScope, reqmethod string) error {
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

// UserHasDeviceNetworkWriteAccess reports whether the user may mutate device network
// membership/state (join, leave, exit-node selection). Read-only network roles that
// only grant Read are denied; Network Users (VPNaccess / extclient create) and roles
// with host write scopes are allowed.
func UserHasDeviceNetworkWriteAccess(ctx context.Context, user *schema.User, network string) bool {
	if user == nil || user.Username == "" || network == "" {
		return false
	}
	if ctx == nil {
		ctx = context.TODO()
	}
	ctx = db.WithContext(ctx)

	if !logic.UserHasAccessToNetwork(ctx, user, network) {
		return false
	}

	platformRole := &schema.UserRole{ID: user.PlatformRoleID}
	if err := platformRole.GetPlatformRole(ctx); err != nil {
		return false
	}
	if platformRole.ID == schema.Auditor {
		return false
	}
	if platformRole.TenantGlobalAccess && !PlatformRoleRequiresGroupEnforcement(user.PlatformRoleID) {
		return true
	}

	networkName := network
	net := &schema.Network{ID: network, Name: network}
	if err := net.Get(ctx); err == nil && net.Name != "" {
		networkName = net.Name
	}
	netID := schema.NetworkID(networkName)

	for groupID := range user.UserGroups.Data() {
		userG, err := GetUserGroup(ctx, groupID)
		if err != nil {
			continue
		}
		roles := userG.NetworkRoles.Data()
		if netRoles, ok := roles[schema.AllNetworks]; ok {
			for netRoleID := range netRoles {
				if networkRoleGrantsDeviceWrite(ctx, netRoleID) {
					return true
				}
			}
		}
		if netRoles, ok := roles[netID]; ok {
			for netRoleID := range netRoles {
				if networkRoleGrantsDeviceWrite(ctx, netRoleID) {
					return true
				}
			}
		}
	}
	return false
}

func networkRoleGrantsDeviceWrite(ctx context.Context, netRoleID schema.UserRoleID) bool {
	role := &schema.UserRole{ID: netRoleID}
	if err := role.GetNetworkRole(ctx); err != nil {
		return false
	}
	return roleGrantsDeviceWrite(role)
}

func roleGrantsDeviceWrite(role *schema.UserRole) bool {
	if role == nil {
		return false
	}
	if role.TenantGlobalAccess {
		return true
	}
	access := role.NetworkLevelAccess.Data()
	if hostScopes, ok := access[schema.HostRsrc]; ok {
		for _, s := range hostScopes {
			if s.Create || s.Update || s.Delete {
				return true
			}
		}
	}
	if ragScopes, ok := access[schema.RemoteAccessGwRsrc]; ok {
		for _, s := range ragScopes {
			if s.VPNaccess {
				return true
			}
		}
	}
	if ecScopes, ok := access[schema.ExtClientsRsrc]; ok {
		for _, s := range ecScopes {
			if s.Create || s.Update || s.Delete {
				return true
			}
		}
	}
	return false
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

func GetSaaSNMUIHost() string {
	switch servercfg.GetEnvironment() {
	case "dev":
		return saasNMUIHostDevelopment
	case "staging":
		return saasNMUIHostStaging
	default:
		return saasNMUIHostProduction
	}
}

func GetSaaSNMUIHostWithVersion() string {
	return fmt.Sprintf("%s/%s", GetSaaSNMUIHost(), servercfg.GetVersion())
}

// CheckUIHostReadAccess ensures a dashboard user may read posture data for a host
// by verifying network-scoped host read permission on at least one host network.
func CheckUIHostReadAccess(r *http.Request, host *schema.Host) error {
	username := r.Header.Get("user")
	if username == logic.MasterUser {
		return nil
	}
	user := &schema.User{Username: username}
	if err := user.Get(r.Context()); err != nil {
		return err
	}
	userRole := &schema.UserRole{ID: user.PlatformRoleID}
	if err := userRole.GetPlatformRole(r.Context()); err != nil {
		return errors.New("access denied")
	}
	if userRole.TenantGlobalAccess || userRole.ID == schema.Auditor {
		return nil
	}

	return errors.New("access denied")
}
