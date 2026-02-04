//go:build ee
// +build ee

package pro

import (
	"context"
	"sync"
	"time"

	ch "github.com/gravitl/netmaker/clickhouse"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/pro/auth"
	proControllers "github.com/gravitl/netmaker/pro/controllers"
	"github.com/gravitl/netmaker/pro/email"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

// InitPro - Initialize Pro Logic
func InitPro() {
	servercfg.IsPro = true
	models.SetLogo(retrieveProLogo())
	controller.HttpMiddlewares = append(
		controller.HttpMiddlewares,
		// TODO: try to add clickhouse middleware only if needed.
		ch.Middleware,
		proControllers.OnlyServerAPIWhenUnlicensedMiddleware,
	)
	controller.HttpHandlers = append(
		controller.HttpHandlers,
		proControllers.MetricHandlers,
		proControllers.UserHandlers,
		proControllers.FailOverHandlers,
		proControllers.RacHandlers,
		proControllers.EventHandlers,
		proControllers.TagHandlers,
		proControllers.NetworkHandlers,
		proControllers.AutoRelayHandlers,
		// TODO: try to add flow handler only if flow logs are enabled.
		proControllers.FlowHandlers,
		proControllers.PostureCheckHandlers,
		proControllers.JITHandlers,
	)
	controller.ListRoles = proControllers.ListRoles
	logic.EnterpriseCheckFuncs = append(logic.EnterpriseCheckFuncs, func(ctx context.Context, wg *sync.WaitGroup) {
		// == License Handling ==
		enableLicenseHook := true
		// licenseKeyValue := servercfg.GetLicenseKey()
		// netmakerTenantID := servercfg.GetNetmakerTenantID()
		// if licenseKeyValue != "" && netmakerTenantID != "" {
		// 	enableLicenseHook = true
		// }
		if !enableLicenseHook {
			err := initTrial()
			if err != nil {
				logger.Log(0, "failed to init trial", err.Error())
				enableLicenseHook = true
			}
			trialEndDate, err := getTrialEndDate()
			if err != nil {
				slog.Error("failed to get trial end date", "error", err)
				enableLicenseHook = true
			} else {
				// check if trial ended
				if time.Now().After(trialEndDate) {
					// trial ended already
					enableLicenseHook = true
				}
			}

		}

		if enableLicenseHook {
			logger.Log(0, "starting license checker")
			ClearLicenseCache()
			if err := ValidateLicense(); err != nil {
				slog.Error(err.Error())
				return
			}
			logger.Log(0, "proceeding with Paid Tier license")
			logic.SetFreeTierForTelemetry(false)
			// == End License Handling ==
			AddLicenseHooks()
		} else {
			logger.Log(0, "starting trial license hook")
			addTrialLicenseHook()
		}

		//AddUnauthorisedUserNodeHooks()

		var authProvider = auth.InitializeAuthProvider()
		if authProvider != "" {
			slog.Info("OAuth provider,", authProvider+",", "initialized")
		} else {
			slog.Error("no OAuth provider found or not configured, continuing without OAuth")
		}
		proLogic.LoadNodeMetricsToCache()
		proLogic.InitFailOverCache()
		if servercfg.CacheEnabled() {
			proLogic.InitAutoRelayCache()
		}
		auth.ResetIDPSyncHook()
		email.Init()
		go proLogic.EventWatcher()
		logic.GetMetricsMonitor().Start()
		proLogic.AddPostureCheckHook()
		// Register JIT expiry hook with email notifications
		addJitExpiryHookWithEmail()
		if proLogic.GetFeatureFlags().EnableFlowLogs && logic.GetServerSettings().EnableFlowLogs {
			err := ch.Initialize()
			if err != nil {
				logger.FatalLog("error connecting to clickhouse:", err.Error())
			}

			proLogic.StartFlowCleanupLoop()

			wg.Add(1)
			go func(ctx context.Context, wg *sync.WaitGroup) {
				<-ctx.Done()
				proLogic.StopFlowCleanupLoop()
				ch.Close()
				wg.Done()
			}(ctx, wg)
		}
	})
	logic.ResetFailOver = proLogic.ResetFailOver
	logic.ResetFailedOverPeer = proLogic.ResetFailedOverPeer
	logic.FailOverExists = proLogic.FailOverExists
	logic.CreateFailOver = proLogic.CreateFailOver
	logic.GetFailOverPeerIps = proLogic.GetFailOverPeerIps

	logic.ResetAutoRelay = proLogic.ResetAutoRelay
	logic.ResetAutoRelayedPeer = proLogic.ResetAutoRelayedPeer
	logic.SetAutoRelay = proLogic.SetAutoRelay
	logic.GetAutoRelayPeerIps = proLogic.GetAutoRelayPeerIps

	logic.DenyClientNodeAccess = proLogic.DenyClientNode
	logic.IsClientNodeAllowed = proLogic.IsClientNodeAllowed
	logic.AllowClientNodeAccess = proLogic.RemoveDeniedNodeFromClient
	logic.SetClientDefaultACLs = proLogic.SetClientDefaultACLs
	logic.SetClientACLs = proLogic.SetClientACLs
	logic.UpdateProNodeACLs = proLogic.UpdateProNodeACLs
	logic.GetMetrics = proLogic.GetMetrics
	logic.UpdateMetrics = proLogic.UpdateMetrics
	logic.DeleteMetrics = proLogic.DeleteMetrics
	logic.GetTrialEndDate = getTrialEndDate
	mq.UpdateMetrics = proLogic.MQUpdateMetrics
	mq.UpdateMetricsFallBack = proLogic.MQUpdateMetricsFallBack
	logic.GetFilteredNodesByUserAccess = proLogic.GetFilteredNodesByUserAccess
	logic.CreateRole = proLogic.CreateRole
	logic.UpdateRole = proLogic.UpdateRole
	logic.DeleteRole = proLogic.DeleteRole
	logic.NetworkPermissionsCheck = proLogic.NetworkPermissionsCheck
	logic.GlobalPermissionsCheck = proLogic.GlobalPermissionsCheck
	logic.DeleteNetworkRoles = proLogic.DeleteNetworkRoles
	logic.CreateDefaultNetworkRolesAndGroups = proLogic.CreateDefaultNetworkRolesAndGroups
	logic.FilterNetworksByRole = proLogic.FilterNetworksByRole
	logic.IsGroupsValid = proLogic.IsGroupsValid
	logic.IsGroupValid = proLogic.IsGroupValid
	logic.IsNetworkRolesValid = proLogic.IsNetworkRolesValid
	logic.InitialiseRoles = proLogic.UserRolesInit
	logic.UpdateUserGwAccess = proLogic.UpdateUserGwAccess
	logic.CreateDefaultUserPolicies = proLogic.CreateDefaultUserPolicies
	logic.MigrateUserRoleAndGroups = proLogic.MigrateUserRoleAndGroups
	logic.MigrateToUUIDs = proLogic.MigrateToUUIDs
	logic.IntialiseGroups = proLogic.UserGroupsInit
	logic.AddGlobalNetRolesToAdmins = proLogic.AddGlobalNetRolesToAdmins
	logic.ListUserGroups = proLogic.ListUserGroups
	logic.GetUserGroupsInNetwork = proLogic.GetUserGroupsInNetwork
	logic.GetUserGroup = proLogic.GetUserGroup
	logic.GetNodeStatus = proLogic.GetNodeStatus
	logic.IsOAuthConfigured = auth.IsOAuthConfigured
	logic.ResetAuthProvider = auth.ResetAuthProvider
	logic.ResetIDPSyncHook = auth.ResetIDPSyncHook
	logic.EmailInit = email.Init
	logic.LogEvent = proLogic.LogEvent
	logic.RemoveUserFromAclPolicy = proLogic.RemoveUserFromAclPolicy
	logic.IsUserAllowedToCommunicate = proLogic.IsUserAllowedToCommunicate
	logic.DeleteAllNetworkTags = proLogic.DeleteAllNetworkTags
	logic.CreateDefaultTags = proLogic.CreateDefaultTags
	logic.IsPeerAllowed = proLogic.IsPeerAllowed
	logic.IsAclPolicyValid = proLogic.IsAclPolicyValid
	logic.GetEgressUserRulesForNode = proLogic.GetEgressUserRulesForNode
	logic.GetTagMapWithNodesByNetwork = proLogic.GetTagMapWithNodesByNetwork
	logic.GetUserAclRulesForNode = proLogic.GetUserAclRulesForNode
	logic.CheckIfAnyPolicyisUniDirectional = proLogic.CheckIfAnyPolicyisUniDirectional
	logic.MigrateToGws = proLogic.MigrateToGws
	logic.GetFwRulesForNodeAndPeerOnGw = proLogic.GetFwRulesForNodeAndPeerOnGw
	logic.GetFwRulesForUserNodesOnGw = proLogic.GetFwRulesForUserNodesOnGw
	logic.GetFeatureFlags = proLogic.GetFeatureFlags
	logic.GetDeploymentMode = proLogic.GetDeploymentMode
	logic.GetNameserversForHost = proLogic.GetNameserversForHost
	logic.GetNameserversForNode = proLogic.GetNameserversForNode
	logic.ValidateNameserverReq = proLogic.ValidateNameserverReq
	logic.ValidateEgressReq = proLogic.ValidateEgressReq
	logic.CheckPostureViolations = proLogic.CheckPostureViolations
	logic.GetPostureCheckDeviceInfoByNode = proLogic.GetPostureCheckDeviceInfoByNode
	logic.StartFlowCleanupLoop = proLogic.StartFlowCleanupLoop
	logic.StopFlowCleanupLoop = proLogic.StopFlowCleanupLoop
	// Expose JIT functions
	logic.CheckJITAccess = proLogic.CheckJITAccess
	logic.AssignVirtualRangeToEgress = proLogic.AssignVirtualRangeToEgress
}

// addJitExpiryHookWithEmail - registers a hook that expires JIT grants and sends email notifications
func addJitExpiryHookWithEmail() {
	if !proLogic.GetFeatureFlags().EnableJIT {
		return
	}
	// Register JIT grant expiry hook with email notifications - runs every 5 minutes
	logic.HookManagerCh <- models.HookDetails{
		ID:       "jit-expiry-hook",
		Hook:     logic.WrapHook(expireJITGrantsWithEmail),
		Interval: 5 * time.Minute,
	}
}

// expireJITGrantsWithEmail - expires JIT grants and sends email notifications
func expireJITGrantsWithEmail() error {
	ctx := db.WithContext(context.Background())

	// Get grants that are about to expire or just expired (within last 5 minutes)
	// We check before expiring so we can send emails
	grant := schema.JITGrant{}
	allGrants, err := grant.ListExpired(ctx)
	if err != nil {
		slog.Warn("failed to list expired grants for email notification", "error", err)
		// Continue with expiration even if listing fails
	}

	// Track grants that need email before we expire them
	var grantsToEmail []struct {
		Grant   schema.JITGrant
		Request schema.JITRequest
		Network models.Network
	}

	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	for _, expiredGrant := range allGrants {
		// Only send email for grants that expired recently (within last 5 minutes)
		if expiredGrant.ExpiresAt.After(fiveMinutesAgo) && expiredGrant.RequestID != "" {
			request := schema.JITRequest{ID: expiredGrant.RequestID}
			if err := request.Get(ctx); err == nil {
				network, err := logic.GetNetwork(expiredGrant.NetworkID)
				if err == nil {
					grantsToEmail = append(grantsToEmail, struct {
						Grant   schema.JITGrant
						Request schema.JITRequest
						Network models.Network
					}{expiredGrant, request, network})
				}
			}
		}
	}

	// First, expire the grants (this handles the business logic)
	if err := proLogic.ExpireJITGrants(); err != nil {
		return err
	}

	// Then, send email notifications for the grants we tracked
	for _, item := range grantsToEmail {
		if err := email.SendJITExpirationEmail(&item.Grant, &item.Request, item.Network, false); err != nil {
			slog.Warn("failed to send expiration email", "grant_id", item.Grant.ID, "user", item.Request.UserName, "error", err)
		}
	}

	return nil
}

func retrieveProLogo() string {
	return `              
 __   __     ______     ______   __    __     ______     __  __     ______     ______    
/\ "-.\ \   /\  ___\   /\__  _\ /\ "-./  \   /\  __ \   /\ \/ /    /\  ___\   /\  == \   
\ \ \-.  \  \ \  __\   \/_/\ \/ \ \ \-./\ \  \ \  __ \  \ \  _"-.  \ \  __\   \ \  __<   
 \ \_\\"\_\  \ \_____\    \ \_\  \ \_\ \ \_\  \ \_\ \_\  \ \_\ \_\  \ \_____\  \ \_\ \_\ 
  \/_/ \/_/   \/_____/     \/_/   \/_/  \/_/   \/_/\/_/   \/_/\/_/   \/_____/   \/_/ /_/ 
                                                                                         																							 
                                   ___    ___   ____                        
           ____  ____  ____       / _ \  / _ \ / __ \       ____  ____  ____
          /___/ /___/ /___/      / ___/ / , _// /_/ /      /___/ /___/ /___/
         /___/ /___/ /___/      /_/    /_/|_| \____/      /___/ /___/ /___/ 
                                                                            
`
}
