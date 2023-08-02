package controller

import (
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// limit consts
const (
	nodesLimit = iota
	networksLimit
	usersLimit
	clientsLimit
)

func checkFreeTierLimits(limitChoice int, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: "free tier limits exceeded on networks",
		}

		if logic.Free_Tier { // check that free tier limits not exceeded
			switch limitChoice {
			case networksLimit:
				currentNetworks, err := logic.GetNetworks()
				if (err != nil && !database.IsEmptyRecord(err)) || len(currentNetworks) >= logic.Networks_Limit {
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case usersLimit:
				users, err := logic.GetUsers()
				if (err != nil && !database.IsEmptyRecord(err)) || len(users) >= logic.Users_Limit {
					errorResponse.Message = "free tier limits exceeded on users"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case clientsLimit:
				clients, err := logic.GetAllExtClients()
				if (err != nil && !database.IsEmptyRecord(err)) || len(clients) >= logic.Clients_Limit {
					errorResponse.Message = "free tier limits exceeded on external clients"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}
