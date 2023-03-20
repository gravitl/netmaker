package logic

import "github.com/gravitl/netmaker/models"

// DenyClientNode - add a denied node to an ext client's list
func DenyClientNode(ec *models.ExtClient, clientOrNodeID string) (ok bool) {
	if ec == nil || len(clientOrNodeID) == 0 {
		return
	}
	if ec.ACLs == nil {
		ec.ACLs = map[string]struct{}{}
	}
	ok = true
	ec.ACLs[clientOrNodeID] = struct{}{}
	return
}

// IsClientNodeAllowed - checks if given ext client and node are allowed to communicate
func IsClientNodeAllowed(ec *models.ExtClient, clientOrNodeID string) bool {
	if ec == nil || len(clientOrNodeID) == 0 {
		return false
	}
	if ec.ACLs == nil {
		return true
	}
	_, ok := ec.ACLs[clientOrNodeID]
	return ok
}

// RemoveDeniedNodeFromClient - removes a node id from set of denied nodes
func RemoveDeniedNodeFromClient(ec *models.ExtClient, clientOrNodeID string) bool {
	if ec.ACLs == nil {
		return true
	}
	_, ok := ec.ACLs[clientOrNodeID]
	if !ok {
		return false
	}
	delete(ec.ACLs, clientOrNodeID)
	return true
}
