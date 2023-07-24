package logic

import "github.com/gravitl/netmaker/models"

// DenyClientNode - add a denied node to an ext client's list
func DenyClientNode(ec *models.ExtClient, clientOrNodeID string) (ok bool) {
	if ec == nil || len(clientOrNodeID) == 0 {
		return
	}
	if ec.DeniedACLs == nil {
		ec.DeniedACLs = map[string]struct{}{}
	}
	ok = true
	ec.DeniedACLs[clientOrNodeID] = struct{}{}
	return
}

// IsClientNodeAllowed - checks if given ext client and node are allowed to communicate
func IsClientNodeAllowed(ec *models.ExtClient, clientOrNodeID string) bool {
	if ec == nil || len(clientOrNodeID) == 0 {
		return false
	}
	if ec.DeniedACLs == nil {
		return true
	}
	_, ok := ec.DeniedACLs[clientOrNodeID]
	return ok
}

// RemoveDeniedNodeFromClient - removes a node id from set of denied nodes
func RemoveDeniedNodeFromClient(ec *models.ExtClient, clientOrNodeID string) bool {
	if ec.DeniedACLs == nil {
		return true
	}
	_, ok := ec.DeniedACLs[clientOrNodeID]
	if !ok {
		return false
	}
	delete(ec.DeniedACLs, clientOrNodeID)
	return true
}
