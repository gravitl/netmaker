package functions

import (
	"fmt"
	"github.com/gravitl/netmaker/models"
	"net/http"
)

func CreateGateway(ingressRequest models.IngressRequest, relayRequest models.RelayRequest) *models.ApiNode {
	return request[models.ApiNode](http.MethodPost, fmt.Sprintf("/api/nodes/%s/%s/gateway", relayRequest.NetID, relayRequest.NodeID), &models.CreateGwReq{
		IngressRequest: ingressRequest,
		RelayRequest:   relayRequest,
	})
}

func DeleteGateway(networkID, nodeID string) *models.ApiNode {
	return request[models.ApiNode](http.MethodDelete, fmt.Sprintf("/api/nodes/%s/%s/gateway", networkID, nodeID), nil)
}
