//go:build ee
// +build ee

package ee

import (
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/ee/ee_controllers"
	eelogic "github.com/gravitl/netmaker/ee/logic"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

// InitEE - Initialize EE Logic
func InitEE() {
	setIsEnterprise()
	servercfg.Is_EE = true
	models.SetLogo(retrieveEELogo())
	controller.HttpMiddlewares = append(
		controller.HttpMiddlewares,
		ee_controllers.OnlyServerAPIWhenUnlicensedMiddleware,
	)
	controller.HttpHandlers = append(
		controller.HttpHandlers,
		ee_controllers.MetricHandlers,
		ee_controllers.NetworkUsersHandlers,
		ee_controllers.UserGroupsHandlers,
		ee_controllers.RelayHandlers,
	)
	logic.EnterpriseCheckFuncs = append(logic.EnterpriseCheckFuncs, func() {
		// == License Handling ==
		if err := ValidateLicense(); err != nil {
			slog.Error(err.Error())
			return
		}
		slog.Info("proceeding with Paid Tier license")
		logic.SetFreeTierForTelemetry(false)
		// == End License Handling ==
		AddLicenseHooks()
		resetFailover()
	})
	logic.EnterpriseFailoverFunc = eelogic.SetFailover
	logic.EnterpriseResetFailoverFunc = eelogic.ResetFailover
	logic.EnterpriseResetAllPeersFailovers = eelogic.WipeAffectedFailoversOnly
	logic.DenyClientNodeAccess = eelogic.DenyClientNode
	logic.IsClientNodeAllowed = eelogic.IsClientNodeAllowed
	logic.AllowClientNodeAccess = eelogic.RemoveDeniedNodeFromClient
}

func resetFailover() {
	nets, err := logic.GetNetworks()
	if err == nil {
		for _, net := range nets {
			err = eelogic.ResetFailover(net.NetID)
			if err != nil {
				slog.Error("failed to reset failover", "network", net.NetID, "error", err.Error())
			}
		}
	}
}

func retrieveEELogo() string {
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
