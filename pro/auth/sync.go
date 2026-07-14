package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/pro/idp"
	"github.com/gravitl/netmaker/pro/idp/azure"
	"github.com/gravitl/netmaker/pro/idp/google"
	"github.com/gravitl/netmaker/pro/idp/okta"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// idpSyncStateMtx guards idpSyncLocks and idpSyncErrs below, which track
// per-tenant sync state since each tenant's hook runs independently and
// concurrently.
var (
	idpSyncStateMtx sync.Mutex
	idpSyncLocks    = make(map[string]*sync.Mutex)
	idpSyncErrs     = make(map[string]error)
)

func idpSyncHookID(ctx context.Context) string {
	return fmt.Sprintf("idp-sync-%s", scope.ID(ctx))
}

func tenantSyncLock(tenantID string) *sync.Mutex {
	idpSyncStateMtx.Lock()
	defer idpSyncStateMtx.Unlock()
	mtx, ok := idpSyncLocks[tenantID]
	if !ok {
		mtx = &sync.Mutex{}
		idpSyncLocks[tenantID] = mtx
	}
	return mtx
}

func idpSyncInterval(settings schema.TenantSettings) time.Duration {
	interval, err := time.ParseDuration(settings.IDPSyncInterval)
	if err != nil || interval == 0 {
		return 24 * time.Hour
	}
	return interval
}

// ResetIDPSyncHook re-reads the settings for the tenant carried in ctx and
// either (re)registers or stops that tenant's idp sync hook accordingly.
func ResetIDPSyncHook(ctx context.Context) {
	if !servercfg.IsMasterPod() {
		if servercfg.IsHA() && logic.PublishServerSync != nil {
			logic.PublishServerSync(ctx, logic.SyncTypeIDPSync)
		}
		return
	}

	StartIDPSyncHookForTenant(ctx)
}

func StartIDPSyncHookForTenant(ctx context.Context) {
	hookID := idpSyncHookID(ctx)

	// Embed scope/db into a fresh context so the hook goroutine carries its
	// own tenant identity independently of any caller's request lifetime.
	tenantCtx := scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx))

	settingsRecord := &schema.TenantSettingsRecord{Key: scope.ID(ctx)}
	if err := settingsRecord.Get(tenantCtx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		logger.Log(0, "failed to load settings for tenant ", scope.ID(ctx), ": ", err.Error())
		return
	}
	settings := settingsRecord.Value.Data()

	if !settings.SyncEnabled {
		logic.StopHook(hookID)
		return
	}

	logic.HookManagerCh <- models.HookDetails{
		ID: hookID,
		Hook: logic.WrapHook(func() error {
			return SyncFromIDP(tenantCtx)
		}),
		Interval: idpSyncInterval(settings),
	}
}

func SyncFromIDP(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	mtx := tenantSyncLock(tenantID)
	mtx.Lock()
	defer mtx.Unlock()

	settingsRecord := &schema.TenantSettingsRecord{
		Key: tenantID,
	}
	var err error
	defer func() {
		idpSyncStateMtx.Lock()
		idpSyncErrs[tenantID] = err
		idpSyncStateMtx.Unlock()
	}()

	err = settingsRecord.Get(ctx)
	if err != nil {
		return err
	}

	settings := settingsRecord.Value.Data()

	var idpClient idp.Client
	var idpUsers []idp.User
	var idpGroups []idp.Group

	switch settings.AuthProvider {
	case "google":
		idpClient, err = google.NewGoogleWorkspaceClient(settings.GoogleAdminEmail, settings.GoogleSACredsJson)
		if err != nil {
			return err
		}
	case "azure-ad":
		idpClient = azure.NewAzureEntraIDClient(settings.ClientID, settings.ClientSecret, settings.AzureTenant)
	case "okta":
		idpClient, err = okta.NewOktaClient(settings.OktaOrgURL, settings.OktaAPIToken)
		if err != nil {
			return err
		}
	default:
		if settings.AuthProvider != "" {
			err = fmt.Errorf("invalid auth provider: %s", settings.AuthProvider)
			return err
		}
	}

	if settings.AuthProvider != "" && idpClient != nil {
		idpUsers, err = idpClient.GetUsers(settings.UserFilters)
		if err != nil {
			return err
		}

		idpGroups, err = idpClient.GetGroups(settings.GroupFilters)
		if err != nil {
			return err
		}

		if len(settings.GroupFilters) > 0 {
			idpUsers = filterUsersByGroupMembership(idpUsers, idpGroups)
		}

		if len(settings.UserFilters) > 0 {
			idpGroups = filterGroupsByMembers(idpGroups, idpUsers)
		}
	}

	err = syncUsers(ctx, idpUsers, settings.UserFilters, settings.AuthProvider == "")
	if err != nil {
		return err
	}

	err = syncGroups(ctx, idpGroups, settings.GroupFilters)
	return err
}

func syncUsers(ctx context.Context, idpUsers []idp.User, filters []string, removeIntegration bool) error {
	dbUsers, err := (&schema.User{}).ListAll(ctx)
	if err != nil {
		return err
	}

	password, err := logic.FetchOAuthSecret()
	if err != nil {
		return err
	}

	idpUsersMap := make(map[string]struct{})
	for _, user := range idpUsers {
		idpUsersMap[user.Username] = struct{}{}
	}

	dbUsersMap := make(map[string]*schema.User)
	for _, user := range dbUsers {
		dbUsersMap[user.Username] = &user
	}

	for _, user := range idpUsers {
		if user.AccountArchived {
			// delete the user if it has been archived.
			user, ok := dbUsersMap[user.Username]
			if ok {
				_ = deleteAndCleanUpUser(ctx, user)
			}
			continue
		}

		var found bool
		for _, filter := range filters {
			if strings.HasPrefix(user.Username, filter) {
				found = true
				break
			}
		}

		// if there are filters but none of them match, then skip this user.
		if len(filters) > 0 && !found {
			continue
		}

		dbUser, ok := dbUsersMap[user.Username]
		if !ok {
			// create the user only if it doesn't exist.
			err = orchestrator.GetRepository().UserOrchestrator().CreateUser(ctx, &schema.User{
				Username:                   user.Username,
				ExternalIdentityProviderID: user.ID,
				DisplayName:                user.DisplayName,
				AccountDisabled:            user.AccountDisabled,
				Password:                   password,
				AuthType:                   schema.OAuth,
				PlatformRoleID:             schema.ServiceUser,
			})
			if err != nil {
				return err
			}

			// It's possible that a user can attempt to log in to Netmaker
			// after the IDP is configured but before the users are synced.
			// Since the user doesn't exist, a pending user will be
			// created. Now, since the user is created, the pending user
			// can be deleted.
			_ = logic.DeletePendingUser(user.Username)
		} else if dbUser.AuthType == schema.OAuth {
			if dbUser.PlatformRoleID != schema.SuperAdminRole &&
				(dbUser.AccountDisabled != user.AccountDisabled ||
					dbUser.DisplayName != user.DisplayName ||
					dbUser.ExternalIdentityProviderID != user.ID) {

				dbUser.AccountDisabled = user.AccountDisabled
				dbUser.DisplayName = user.DisplayName
				dbUser.ExternalIdentityProviderID = user.ID

				err = logic.UpsertUser(*dbUser)
				if err != nil {
					return err
				}
			}
		} else {
			logger.Log(0, "user with username "+user.Username+" already exists, skipping creation")
			continue
		}
	}

	for _, user := range dbUsersMap {
		if user.ExternalIdentityProviderID != "" {
			if _, ok := idpUsersMap[user.Username]; !ok {
				if user.PlatformRoleID == schema.SuperAdminRole && !removeIntegration {
					continue
				}

				// delete the user if it has been deleted on idp
				// or is filtered out.
				err = deleteAndCleanUpUser(ctx, user)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func syncGroups(ctx context.Context, idpGroups []idp.Group, filters []string) error {
	dbGroups, err := (&schema.UserGroup{}).ListAll(ctx)
	if err != nil {
		return err
	}

	dbUsers, err := (&schema.User{}).ListAll(ctx)
	if err != nil {
		return err
	}

	idpGroupsMap := make(map[string]struct{})
	for _, group := range idpGroups {
		idpGroupsMap[group.ID] = struct{}{}
	}

	dbGroupsMap := make(map[string]schema.UserGroup)
	for _, group := range dbGroups {
		if group.ExternalIdentityProviderID != "" {
			dbGroupsMap[group.ExternalIdentityProviderID] = group
		}
	}

	dbUsersMap := make(map[string]*schema.User)
	for _, user := range dbUsers {
		if user.ExternalIdentityProviderID != "" {
			dbUsersMap[user.ExternalIdentityProviderID] = &user
		}
	}

	modifiedUsers := make(map[string]struct{})

	for _, group := range idpGroups {
		var found bool
		for _, filter := range filters {
			if strings.HasPrefix(group.Name, filter) {
				found = true
				break
			}
		}

		// if there are filters but none of them match, then skip this group.
		if len(filters) > 0 && !found {
			continue
		}

		dbGroup, ok := dbGroupsMap[group.ID]
		if !ok {
			dbGroup.ExternalIdentityProviderID = group.ID
			dbGroup.Name = group.Name
			dbGroup.Default = false
			dbGroup.NetworkRoles = datatypes.NewJSONType(schema.NetworkRoles{})
			err := proLogic.CreateUserGroup(ctx, &dbGroup)
			if err != nil {
				return err
			}
		} else {
			dbGroup.Name = group.Name
			err = proLogic.UpdateUserGroup(dbGroup)
			if err != nil {
				return err
			}
		}

		groupMembersMap := make(map[string]struct{})
		for _, member := range group.Members {
			groupMembersMap[member] = struct{}{}
		}

		for _, user := range dbUsers {
			// use dbGroup.Name because the group name may have been changed on idp.
			_, inNetmakerGroup := user.UserGroups.Data()[dbGroup.ID]
			_, inIDPGroup := groupMembersMap[user.ExternalIdentityProviderID]

			if inNetmakerGroup && !inIDPGroup {
				// use dbGroup.Name because the group name may have been changed on idp.
				delete(dbUsersMap[user.ExternalIdentityProviderID].UserGroups.Data(), dbGroup.ID)
				modifiedUsers[user.ExternalIdentityProviderID] = struct{}{}
			}

			if !inNetmakerGroup && inIDPGroup {
				// use dbGroup.Name because the group name may have been changed on idp.
				dbUsersMap[user.ExternalIdentityProviderID].UserGroups.Data()[dbGroup.ID] = struct{}{}
				modifiedUsers[user.ExternalIdentityProviderID] = struct{}{}
			}
		}
	}

	for userID := range modifiedUsers {
		user, ok := dbUsersMap[userID]
		if ok {
			err = logic.UpsertUser(*user)
			if err != nil {
				return err
			}
		}
	}

	for _, group := range dbGroups {
		if group.ExternalIdentityProviderID != "" {
			if _, ok := idpGroupsMap[group.ExternalIdentityProviderID]; !ok {
				// delete the group if it has been deleted on idp
				// or is filtered out.
				err = proLogic.DeleteAndCleanUpGroup(&group)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func GetIDPSyncStatus(ctx context.Context) models.IDPSyncStatus {
	tenantID := scope.ID(ctx)
	mtx := tenantSyncLock(tenantID)
	if mtx.TryLock() {
		defer mtx.Unlock()
		idpSyncStateMtx.Lock()
		err := idpSyncErrs[tenantID]
		idpSyncStateMtx.Unlock()
		if err == nil {
			return models.IDPSyncStatus{
				Status: "completed",
			}
		}
		return models.IDPSyncStatus{
			Status:      "failed",
			Description: err.Error(),
		}
	}
	return models.IDPSyncStatus{
		Status: "in_progress",
	}
}

func filterUsersByGroupMembership(idpUsers []idp.User, idpGroups []idp.Group) []idp.User {
	usersMap := make(map[string]int)
	for i, user := range idpUsers {
		usersMap[user.ID] = i
	}

	filteredUsersMap := make(map[string]int)
	for _, group := range idpGroups {
		for _, member := range group.Members {
			if userIdx, ok := usersMap[member]; ok {
				// user at index `userIdx` is a member of at least one of the
				// groups in the `idpGroups` list, so we keep it.
				filteredUsersMap[member] = userIdx
			}
		}
	}

	i := 0
	filteredUsers := make([]idp.User, len(filteredUsersMap))
	for _, userIdx := range filteredUsersMap {
		filteredUsers[i] = idpUsers[userIdx]
		i++
	}

	return filteredUsers
}

func filterGroupsByMembers(idpGroups []idp.Group, idpUsers []idp.User) []idp.Group {
	usersMap := make(map[string]int)
	for i, user := range idpUsers {
		usersMap[user.ID] = i
	}

	filteredGroupsMap := make(map[int]bool)
	for i, group := range idpGroups {
		var members []string
		for _, member := range group.Members {
			if _, ok := usersMap[member]; ok {
				members = append(members, member)
			}
		}

		if len(members) > 0 {
			// the group at index `i` has members from the `idpUsers` list,
			// so we keep it.
			filteredGroupsMap[i] = true
			// filter out members that were not provided in the `idpUsers` list.
			idpGroups[i].Members = members
		}
	}

	i := 0
	filteredGroups := make([]idp.Group, len(filteredGroupsMap))
	for groupIdx := range filteredGroupsMap {
		filteredGroups[i] = idpGroups[groupIdx]
		i++
	}

	return filteredGroups
}

// TODO: deduplicate
// The cyclic import between the package logic and mq requires this
// function to be duplicated in multiple places.
func deleteAndCleanUpUser(ctx context.Context, user *schema.User) error {
	err := logic.DeleteUser(ctx, user)
	if err != nil {
		return err
	}

	// check and delete extclient with this ownerID
	go func(ctx context.Context) {
		extclients, err := logic.GetAllExtClients(ctx)
		if err != nil {
			return
		}
		for _, extclient := range extclients {
			if extclient.OwnerID == user.Username {
				err = logic.DeleteExtClientAndCleanup(ctx, extclient)
				if err == nil {
					_ = mq.PublishDeletedClientPeerUpdate(ctx, &extclient)
				}
			}
		}

		go logic.DeleteUserInvite(user.Username)
		go mq.PublishPeerUpdate(ctx, false)
	}(scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx)))

	return nil
}
