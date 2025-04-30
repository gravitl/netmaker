package functions

import (
	"fmt"
	"github.com/gravitl/netmaker/logic/nodeacls"
	"net/http"
)

// GetACL - fetch all ACLs associated with a network
func GetACL(networkName string) *nodeacls.ACLContainer {
	return request[nodeacls.ACLContainer](http.MethodGet, fmt.Sprintf("/api/networks/%s/acls", networkName), nil)
}

// UpdateACL - update an ACL
func UpdateACL(networkName string, payload *nodeacls.ACLContainer) *nodeacls.ACLContainer {
	return request[nodeacls.ACLContainer](http.MethodPut, fmt.Sprintf("/api/networks/%s/acls/v2", networkName), payload)
}
