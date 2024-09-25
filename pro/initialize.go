//go:build ee
// +build ee

package pro

import (
	"time"

	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/pro/auth"
	proControllers "github.com/gravitl/netmaker/pro/controllers"
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
		proControllers.OnlyServerAPIWhenUnlicensedMiddleware,
	)
	controller.HttpHandlers = append(
		controller.HttpHandlers,
		proControllers.MetricHandlers,
		proControllers.RelayHandlers,
		proControllers.UserHandlers,
		proControllers.FailOverHandlers,
		proControllers.InetHandlers,
	)
	controller.ListRoles = proControllers.ListRoles
	logic.EnterpriseCheckFuncs = append(logic.EnterpriseCheckFuncs, func() {
		// == License Handling ==
		enableLicenseHook := false
		licenseKeyValue := servercfg.GetLicenseKey()
		netmakerTenantID := servercfg.GetNetmakerTenantID()
		if licenseKeyValue != "" && netmakerTenantID != "" {
			enableLicenseHook = true
		}
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

		if servercfg.GetServerConfig().RacAutoDisable {
			AddRacHooks()
		}

		var authProvider = auth.InitializeAuthProvider()
		if authProvider != "" {
			slog.Info("OAuth provider,", authProvider+",", "initialized")
		} else {
			slog.Error("no OAuth provider found or not configured, continuing without OAuth")
		}
		proLogic.LoadNodeMetricsToCache()
	})
	logic.ResetFailOver = proLogic.ResetFailOver
	logic.ResetFailedOverPeer = proLogic.ResetFailedOverPeer
	logic.FailOverExists = proLogic.FailOverExists
	logic.CreateFailOver = proLogic.CreateFailOver
	logic.GetFailOverPeerIps = proLogic.GetFailOverPeerIps
	logic.DenyClientNodeAccess = proLogic.DenyClientNode
	logic.IsClientNodeAllowed = proLogic.IsClientNodeAllowed
	logic.AllowClientNodeAccess = proLogic.RemoveDeniedNodeFromClient
	logic.SetClientDefaultACLs = proLogic.SetClientDefaultACLs
	logic.SetClientACLs = proLogic.SetClientACLs
	logic.UpdateProNodeACLs = proLogic.UpdateProNodeACLs
	logic.GetMetrics = proLogic.GetMetrics
	logic.UpdateMetrics = proLogic.UpdateMetrics
	logic.DeleteMetrics = proLogic.DeleteMetrics
	logic.GetRelays = proLogic.GetRelays
	logic.GetAllowedIpsForRelayed = proLogic.GetAllowedIpsForRelayed
	logic.RelayedAllowedIPs = proLogic.RelayedAllowedIPs
	logic.UpdateRelayed = proLogic.UpdateRelayed
	logic.SetRelayedNodes = proLogic.SetRelayedNodes
	logic.RelayUpdates = proLogic.RelayUpdates
	logic.ValidateRelay = proLogic.ValidateRelay
	logic.GetTrialEndDate = getTrialEndDate
	logic.SetDefaultGw = proLogic.SetDefaultGw
	logic.SetDefaultGwForRelayedUpdate = proLogic.SetDefaultGwForRelayedUpdate
	logic.UnsetInternetGw = proLogic.UnsetInternetGw
	logic.SetInternetGw = proLogic.SetInternetGw
	logic.GetAllowedIpForInetNodeClient = proLogic.GetAllowedIpForInetNodeClient
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
