package controller

import (
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/ee"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
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

		if ee.Limits.FreeTier { // check that free tier limits not exceeded
			if limit_choice == networks_l {
				currentNetworks, err := logic.GetNetworks()
				if (err != nil && !database.IsEmptyRecord(err)) || len(currentNetworks) >= ee.Limits.Networks {
					returnErrorResponse(w, r, errorResponse)
					return
				}
			} else if limit_choice == node_l {
				nodes, err := logic.GetAllNodes()
				if (err != nil && !database.IsEmptyRecord(err)) || len(nodes) >= ee.Limits.Nodes {
					errorResponse.Message = "free tier limits exceeded on nodes"
					returnErrorResponse(w, r, errorResponse)
					return
				}
			} else if limit_choice == users_l {
				users, err := logic.GetUsers()
				if (err != nil && !database.IsEmptyRecord(err)) || len(users) >= ee.Limits.Users {
					errorResponse.Message = "free tier limits exceeded on users"
					returnErrorResponse(w, r, errorResponse)
					return
				}
			} else if limit_choice == clients_l {
				clients, err := logic.GetAllExtClients()
				if (err != nil && !database.IsEmptyRecord(err)) || len(clients) >= ee.Limits.Clients {
					errorResponse.Message = "free tier limits exceeded on external clients"
					returnErrorResponse(w, r, errorResponse)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}
