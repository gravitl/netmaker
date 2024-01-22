//go:build ee
// +build ee

package pro

import (
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
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
	)
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
				logger.FatalLog0("failed to init trail", err.Error())
			}
			// trialEndDate, err := getTrialEndDate()
			// if err != nil {
			// 	slog.Error("failed to get trial end date", "error", err)
			// 	enableLicenseHook = true
			// }
			// // check if trial ended
			// if time.Now().After(trialEndDate) {
			// 	// trial ended already
			// 	enableLicenseHook = true
			// }
		}

		if enableLicenseHook {
			slog.Info("starting license checker")
			ClearLicenseCache()
			if err := ValidateLicense(); err != nil {
				slog.Error(err.Error())
				return
			}
			slog.Info("proceeding with Paid Tier license")
			logic.SetFreeTierForTelemetry(false)
			// == End License Handling ==
			AddLicenseHooks()
		} else {
			addTrialLicenseHook()
		}

		if servercfg.GetServerConfig().RacAutoDisable {
			AddRacHooks()
		}

	})
	logic.ResetFailOver = proLogic.ResetFailOver
	logic.ResetFailedOverPeer = proLogic.ResetFailedOverPeer
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
	logic.IsInternetGw = proLogic.IsInternetGw
	logic.SetInternetGw = proLogic.SetInternetGw
	logic.GetTrialEndDate = getTrialEndDate
	mq.UpdateMetrics = proLogic.MQUpdateMetrics
	mq.UpdateMetricsFallBack = proLogic.MQUpdateMetricsFallBack
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
