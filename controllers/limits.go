package controller

import (
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// limit consts
const (
	limitChoiceNetworks = iota
	limitChoiceUsers
	limitChoiceMachines
	limitChoiceIngress
	limitChoiceEgress
)

func checkFreeTierLimits(limitChoice int, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: "free tier limits exceeded on ",
		}

		if logic.FreeTier { // check that free tier limits not exceeded
			switch limitChoice {
			case limitChoiceNetworks:
				currentNetworks, err := logic.GetNetworks()
				if (err != nil && !database.IsEmptyRecord(err)) ||
					len(currentNetworks) >= logic.NetworksLimit {
					errorResponse.Message += "networks"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceUsers:
				users, err := logic.GetUsers()
				if (err != nil && !database.IsEmptyRecord(err)) ||
					len(users) >= logic.UsersLimit {
					errorResponse.Message += "users"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceMachines:
				hosts, hErr := logic.GetAllHosts()
				clients, cErr := logic.GetAllExtClients()
				if (hErr != nil && !database.IsEmptyRecord(hErr)) ||
					(cErr != nil && !database.IsEmptyRecord(cErr)) ||
					len(hosts)+len(clients) >= logic.MachinesLimit {
					errorResponse.Message += "machines"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceIngress:
				ingresses, err := logic.GetAllIngresses()
				if (err != nil && !database.IsEmptyRecord(err)) ||
					len(ingresses) >= logic.IngressesLimit {
					errorResponse.Message += "ingresses"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceEgress:
				egresses, err := logic.GetAllEgresses()
				if (err != nil && !database.IsEmptyRecord(err)) ||
					len(egresses) >= logic.EgressesLimit {
					errorResponse.Message += "egresses"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}
