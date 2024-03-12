package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// EnableNodeFailover - Enable failover for a given Node
func EnableNodeFailover(nodeID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodPost, fmt.Sprintf("/api/v1/node/%s/failover", nodeID), nil)
}

// DisableNodeFailover - Disable failover for a given Node
func DisableNodeFailover(nodeID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/v1/node/%s/failover", nodeID), nil)
}
