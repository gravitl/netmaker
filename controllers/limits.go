package controller

import (
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// limit consts
const (
	limitChoiceNodes = iota
	limitChoiceNetworks
	limitChoiceUsers
	limitChoiceClients
)

func checkFreeTierLimits(limitChoice int, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: "free tier limits exceeded on networks",
		}

		if logic.FreeTier { // check that free tier limits not exceeded
			switch limitChoice {
			case limitChoiceNetworks:
				currentNetworks, err := logic.GetNetworks()
				if (err != nil && !database.IsEmptyRecord(err)) || len(currentNetworks) >= logic.NetworksLimit {
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceUsers:
				users, err := logic.GetUsers()
				if (err != nil && !database.IsEmptyRecord(err)) || len(users) >= logic.UsersLimit {
					errorResponse.Message = "free tier limits exceeded on users"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceClients:
				clients, err := logic.GetAllExtClients()
				if (err != nil && !database.IsEmptyRecord(err)) || len(clients) >= logic.ClientsLimit {
					errorResponse.Message = "free tier limits exceeded on external clients"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}
