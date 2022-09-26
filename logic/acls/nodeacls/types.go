package nodeacls

import (
	"github.com/netmakerio/netmaker/logic/acls"
)

type (
	// NodeACL - interface for NodeACLs
	NodeACL acls.ACL
	// NodeID - node ID for ACLs
	NodeID acls.AclID
	// NetworkID - ACL container based on network ID for nodes
	NetworkID acls.ContainerID
)
