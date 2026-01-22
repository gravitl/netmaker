//go:build ee
// +build ee

package pro

import (
	"context"
	"sync"
	"time"

	ch "github.com/gravitl/netmaker/clickhouse"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/pro/auth"
	proControllers "github.com/gravitl/netmaker/pro/controllers"
	"github.com/gravitl/netmaker/pro/email"
	proLogic "github.com/gravitl/netmaker/pro/logic"
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

		AddUnauthorisedUserNodeHooks()

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
	logic.GetHostLocInfo = proLogic.GetHostLocInfo
	logic.GetFeatureFlags = proLogic.GetFeatureFlags
	logic.GetNameserversForHost = proLogic.GetNameserversForHost
	logic.GetNameserversForNode = proLogic.GetNameserversForNode
	logic.ValidateNameserverReq = proLogic.ValidateNameserverReq
	logic.ValidateEgressReq = proLogic.ValidateEgressReq
	logic.CheckPostureViolations = proLogic.CheckPostureViolations
	logic.GetPostureCheckDeviceInfoByNode = proLogic.GetPostureCheckDeviceInfoByNode
	logic.StartFlowCleanupLoop = proLogic.StartFlowCleanupLoop
	logic.StopFlowCleanupLoop = proLogic.StopFlowCleanupLoop
	// Expose JIT functions
	logic.EnableJITOnNetworkFunc = proLogic.EnableJITOnNetwork
	logic.DisableJITOnNetworkFunc = proLogic.DisableJITOnNetwork
	logic.ApproveJITRequestFunc = proLogic.ApproveJITRequest
	logic.DenyJITRequestFunc = proLogic.DenyJITRequest
	logic.CheckJITAccessFunc = proLogic.CheckJITAccess
	logic.GetNetworkJITRequestsFunc = func(networkID string, statusFilter string) ([]interface{}, error) {
		result, err := proLogic.GetNetworkJITRequests(networkID, statusFilter)
		if err != nil {
			return nil, err
		}
		// Convert to []interface{} for type compatibility
		interfaces := make([]interface{}, len(result))
		for i, v := range result {
			interfaces[i] = v
		}
		return interfaces, nil
	}
	logic.GetUserJITStatusFunc = proLogic.GetUserJITStatus
	logic.GetUserJITNetworksStatusFunc = func(networks []models.Network, userID string) ([]interface{}, error) {
		result, err := proLogic.GetUserJITNetworksStatus(networks, userID)
		if err != nil {
			return nil, err
		}
		// Convert to []interface{} for type compatibility
		interfaces := make([]interface{}, len(result))
		for i, v := range result {
			interfaces[i] = v
		}
		return interfaces, nil
	}
	logic.ExpireJITGrants = proLogic.ExpireJITGrants
	logic.DisconnectExtClientsFromNetworkFunc = proLogic.DisconnectExtClientsFromNetwork
	logic.GetNetworkAdminsFunc = proLogic.GetNetworkAdmins
	// Set email notification functions to avoid import cycles
	proLogic.NotifyNetworkAdminsOfJITRequestFunc = proControllers.SendJITRequestEmails
	proLogic.NotifyUserOfJITApprovalFunc = proControllers.SendJITApprovalEmail

	// Register JIT grant expiry hook - runs every 5 minutes
	logic.HookManagerCh <- models.HookDetails{
		ID:       "jit-expiry-hook",
		Hook:     logic.WrapHook(proLogic.ExpireJITGrants),
		Interval: 5 * time.Minute,
	}
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
