package controller

import (
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// limit consts
const (
	node_l     = 0
	networks_l = 1
	users_l    = 2
	clients_l  = 3
)

func checkFreeTierLimits(limit_choice int, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "free tier limits exceeded on networks",
		}

		if logic.Free_Tier && servercfg.Is_EE { // check that free tier limits not exceeded
			if limit_choice == networks_l {
				currentNetworks, err := logic.GetNetworks()
				if (err != nil && !database.IsEmptyRecord(err)) || len(currentNetworks) >= logic.Networks_Limit {
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			} else if limit_choice == node_l {
				nodes, err := logic.GetAllNodes()
				if (err != nil && !database.IsEmptyRecord(err)) || len(nodes) >= logic.Node_Limit {
					errorResponse.Message = "free tier limits exceeded on nodes"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			} else if limit_choice == users_l {
				users, err := logic.GetUsers()
				if (err != nil && !database.IsEmptyRecord(err)) || len(users) >= logic.Users_Limit {
					errorResponse.Message = "free tier limits exceeded on users"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			} else if limit_choice == clients_l {
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
