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
// per-scope sync state (keyed by scope level, then scope id) since each
// tenant/org's hook runs independently and concurrently.
var (
	idpSyncStateMtx sync.Mutex
	idpSyncLocks    = make(map[scope.Scope]map[string]*sync.Mutex)
	idpSyncErrs     = make(map[scope.Scope]map[string]error)
)

type idpSyncSettings struct {
	AuthProvider      string
	ClientID          string
	ClientSecret      string
	AzureTenant       string
	GoogleAdminEmail  string
	GoogleSACredsJson string
	OktaOrgURL        string
	OktaAPIToken      string
	UserFilters       []string
	GroupFilters      []string
	SyncEnabled       bool
	IDPSyncInterval   string
}

func loadIDPSyncSettings(ctx context.Context) (idpSyncSettings, error) {
	if scope.Level(ctx) == scope.OrgScope {
		settingsRecord := &schema.OrganizationSettings{ID: scope.ID(ctx)}
		if err := settingsRecord.Get(ctx); err != nil {
			return idpSyncSettings{}, err
		}
		settings := settingsRecord.Settings.Data()
		return idpSyncSettings{
			AuthProvider:      settings.AuthProvider,
			ClientID:          settings.ClientID,
			ClientSecret:      settings.ClientSecret,
			AzureTenant:       settings.AzureTenant,
			GoogleAdminEmail:  settings.GoogleAdminEmail,
			GoogleSACredsJson: settings.GoogleSACredsJson,
			OktaOrgURL:        settings.OktaOrgURL,
			OktaAPIToken:      settings.OktaAPIToken,
			UserFilters:       cleanFilters(settings.UserFilters),
			SyncEnabled:       settings.SyncEnabled,
			IDPSyncInterval:   settings.IDPSyncInterval,
		}, nil
	}

	settingsRecord := &schema.TenantSettingsRecord{Key: scope.ID(ctx)}
	if err := settingsRecord.Get(ctx); err != nil {
		return idpSyncSettings{}, err
	}
	settings := settingsRecord.Value.Data()
	return idpSyncSettings{
		AuthProvider:      settings.AuthProvider,
		ClientID:          settings.ClientID,
		ClientSecret:      settings.ClientSecret,
		AzureTenant:       settings.AzureTenant,
		GoogleAdminEmail:  settings.GoogleAdminEmail,
		GoogleSACredsJson: settings.GoogleSACredsJson,
		OktaOrgURL:        settings.OktaOrgURL,
		OktaAPIToken:      settings.OktaAPIToken,
		UserFilters:       cleanFilters(settings.UserFilters),
		GroupFilters:      cleanFilters(settings.GroupFilters),
		SyncEnabled:       settings.SyncEnabled,
		IDPSyncInterval:   settings.IDPSyncInterval,
	}, nil
}

func cleanFilters(filters []string) []string {
	var cleaned []string
	for _, filter := range filters {
		filter = strings.TrimSpace(filter)
		if filter != "" {
			cleaned = append(cleaned, filter)
		}
	}
	return cleaned
}

func idpSyncHookID(ctx context.Context) string {
	return fmt.Sprintf("idp-sync-%d-%s", scope.Level(ctx), scope.ID(ctx))
}

func syncLock(level scope.Scope, id string) *sync.Mutex {
	idpSyncStateMtx.Lock()
	defer idpSyncStateMtx.Unlock()
	if idpSyncLocks[level] == nil {
		idpSyncLocks[level] = make(map[string]*sync.Mutex)
	}
	mtx, ok := idpSyncLocks[level][id]
	if !ok {
		mtx = &sync.Mutex{}
		idpSyncLocks[level][id] = mtx
	}
	return mtx
}

func setSyncErr(level scope.Scope, id string, err error) {
	idpSyncStateMtx.Lock()
	defer idpSyncStateMtx.Unlock()
	if idpSyncErrs[level] == nil {
		idpSyncErrs[level] = make(map[string]error)
	}
	idpSyncErrs[level][id] = err
}

func getSyncErr(level scope.Scope, id string) error {
	idpSyncStateMtx.Lock()
	defer idpSyncStateMtx.Unlock()
	return idpSyncErrs[level][id]
}

func idpSyncInterval(settings idpSyncSettings) time.Duration {
	interval, err := time.ParseDuration(settings.IDPSyncInterval)
	if err != nil || interval == 0 {
		return 24 * time.Hour
	}
	return interval
}

// ResetIDPSyncHook re-reads the settings for the scope carried in ctx and
// either (re)registers or stops that scope's idp sync hook accordingly.
func ResetIDPSyncHook(ctx context.Context) {
	if !servercfg.IsMasterPod() {
		if servercfg.IsHA() && logic.PublishServerSync != nil {
			logic.PublishServerSync(ctx, logic.SyncTypeIDPSync)
		}
		return
	}

	StartIDPSyncHook(ctx)
}

func StartIDPSyncHook(ctx context.Context) {
	hookID := idpSyncHookID(ctx)

	// Embed scope/db into a fresh context so the hook goroutine carries its
	// own tenant/org identity independently of any caller's request lifetime.
	scopedCtx := scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx))

	settings, err := loadIDPSyncSettings(scopedCtx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		logger.Log(0, "failed to load idp sync settings for ", scope.ID(ctx), ": ", err.Error())
		return
	}

	if !settings.SyncEnabled {
		logic.StopHook(hookID)
		return
	}

	logic.HookManagerCh <- models.HookDetails{
		ID: hookID,
		Hook: logic.WrapHook(func() error {
			return SyncFromIDP(scopedCtx)
		}),
		Interval: idpSyncInterval(settings),
	}
}

func SyncFromIDP(ctx context.Context) error {
	level, id := scope.Level(ctx), scope.ID(ctx)
	mtx := syncLock(level, id)
	mtx.Lock()
	defer mtx.Unlock()

	var err error
	defer func() {
		setSyncErr(level, id, err)
	}()

	settings, err := loadIDPSyncSettings(ctx)
	if err != nil {
		return err
	}

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
		logger.Log(0, "idp sync: fetched", fmt.Sprint(len(idpUsers)), "users from", settings.AuthProvider, "for", scope.ID(ctx), "with filters", fmt.Sprint(settings.UserFilters))

		if scope.Level(ctx) != scope.OrgScope {
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
	}

	err = syncUsers(ctx, idpUsers, settings.UserFilters, settings.AuthProvider == "")
	if err != nil {
		return err
	}

	if scope.Level(ctx) == scope.OrgScope {
		return nil
	}

	err = syncGroups(ctx, idpGroups, settings.GroupFilters)
	return err
}

func syncUsers(ctx context.Context, idpUsers []idp.User, filters []string, removeIntegration bool) error {
	isOrgScope := scope.Level(ctx) == scope.OrgScope

	// topRole is the role that idp sync must never delete or overwrite
	// outside of an explicit integration removal.
	topRole := schema.SuperAdminRole
	deleteUser := logic.DeleteTenantUser
	newUserRole := schema.ServiceUser
	if isOrgScope {
		topRole = schema.OrgOwner
		deleteUser = logic.DeleteOrgUser
		newUserRole = schema.OrgUser
	}

	dbUsers, err := (&schema.User{}).ListAllWithMembership(ctx)
	if err != nil {
		return err
	}

	password, err := logic.FetchOAuthSecret(ctx)
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
				_ = deleteUser(ctx, user, true, cleanupUserRefs)
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
			createErr := orchestrator.GetRepository().UserOrchestrator().CreateUser(ctx, &schema.User{
				Username:                   user.Username,
				ExternalIdentityProviderID: user.ID,
				DisplayName:                user.DisplayName,
				AccountDisabled:            user.AccountDisabled,
				Password:                   password,
				AuthType:                   schema.OAuth,
				PlatformRoleID:             newUserRole,
				EmailValidated:             true,
			})
			if createErr != nil {
				if errors.Is(createErr, logic.ErrUserLimitExceeded) {
					logger.Log(0, "idp sync: skipping user", user.Username, "user limit reached for tenant", scope.ID(ctx))
					continue
				}
				return createErr
			}

			// It's possible that a user can attempt to log in to Netmaker
			// after the IDP is configured but before the users are synced.
			// Since the user doesn't exist, a pending user will be
			// created. Now, since the user is created, the pending user
			// can be deleted.
			_ = (&schema.PendingUser{
				Username: user.Username,
			}).Delete(ctx)
		} else if dbUser.AuthType == schema.OAuth {
			if dbUser.PlatformRoleID != topRole &&
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

				if isOrgScope {
					om := &schema.OrgMembership{
						OrganizationID:             scope.ID(ctx),
						UserID:                     dbUser.ID,
						ExternalIdentityProviderID: user.ID,
					}
					err = om.UpdateExternalIdentityProviderID(ctx)
				} else {
					tm := &schema.TenantMembership{
						TenantID:                   scope.ID(ctx),
						UserID:                     dbUser.ID,
						ExternalIdentityProviderID: user.ID,
					}
					err = tm.UpdateExternalIdentityProviderID(ctx)
				}
				if err != nil {
					return err
				}

				err = dbUser.UpdateAccountStatus(ctx)
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
				if user.PlatformRoleID == topRole && !removeIntegration {
					continue
				}

				// delete the user if it has been deleted on idp
				// or is filtered out.
				err = deleteUser(ctx, user, true, cleanupUserRefs)
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

	dbUsers, err := (&schema.User{}).ListAllWithMembership(ctx)
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
			err = proLogic.UpdateUserGroup(ctx, dbGroup)
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
			tm := &schema.TenantMembership{
				TenantID: scope.ID(ctx),
				UserID:   user.ID,
				Groups:   user.UserGroups,
			}
			err = tm.UpdateGroups(ctx)
			if err != nil {
				return err
			}
		}
	}
	if len(modifiedUsers) > 0 {
		postureCtx := scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx))
		go proLogic.RunPostureChecksForTenant(postureCtx)
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
	level, id := scope.Level(ctx), scope.ID(ctx)
	mtx := syncLock(level, id)
	if mtx.TryLock() {
		defer mtx.Unlock()
		err := getSyncErr(level, id)
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

func cleanupUserRefs(ctx context.Context, username string, forceDeleteConfigs bool) {
	if scope.Level(ctx) != scope.OrgScope {
		extclients, err := logic.GetAllExtClients(ctx)
		if err == nil {
			for _, extclient := range extclients {
				if extclient.OwnerID == username {
					if err := logic.DeleteExtClientAndCleanup(ctx, extclient); err == nil {
						_ = mq.PublishDeletedClientPeerUpdate(ctx, &extclient)
					}
				}
			}
		}

		_ = mq.PublishPeerUpdate(ctx, false)
	}

	_ = (&schema.UserInvite{
		Email: username,
	}).DeleteByEmail(ctx)
}
