package controller

import (
	"context"
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
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
				numNetworks, err := (&schema.Network{}).Count(db.WithContext(context.TODO()))
				if err != nil || numNetworks >= logic.NetworksLimit {
					errorResponse.Message += "networks"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceUsers:
				numUsers, err := (&schema.User{}).Count(db.WithContext(context.TODO()))
				if err != nil || numUsers >= logic.UsersLimit {
					errorResponse.Message += "users"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			case limitChoiceMachines:
				hosts, hErr := (&schema.Host{}).ListAll(r.Context())
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
				numEgresses, err := (&schema.Egress{}).Count(db.WithContext(context.TODO()))
				if err != nil || numEgresses >= logic.EgressesLimit {
					errorResponse.Message += "egresses"
					logic.ReturnErrorResponse(w, r, errorResponse)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}
