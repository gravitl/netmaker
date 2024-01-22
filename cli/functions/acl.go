package functions

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitl/netmaker/logic/acls"
)

// GetACL - fetch all ACLs associated with a network
func GetACL(networkName string) *acls.ACLContainer {
	return request[acls.ACLContainer](http.MethodGet, fmt.Sprintf("/api/networks/%s/acls", url.QueryEscape(networkName)), nil)
}

// UpdateACL - update an ACL
func UpdateACL(networkName string, payload *acls.ACLContainer) *acls.ACLContainer {
	return request[acls.ACLContainer](http.MethodPut, fmt.Sprintf("/api/networks/%s/acls", url.QueryEscape(networkName)), payload)
}
