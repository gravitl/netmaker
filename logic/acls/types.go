package acls

var (
	// NotPresent - 0 - not present (default)
	NotPresent = byte(0)
	// NotAllowed - 1 - not allowed access
	NotAllowed = byte(1) // 1 - not allowed
	// Allowed - 2 - allowed access
	Allowed = byte(2)
)

type (
	// AclID - the node id of a given node
	AclID string

	// ACL - the ACL of other nodes in a NetworkACL for a single unique node
	ACL map[AclID]byte

	// ACLJson - the string representation in JSON of an ACL Node or Network
	ACLJson string

	// ContainerID - the networkID of a given network
	ContainerID string

	// ACLContainer - the total list of all node's ACL in a given network
	ACLContainer map[AclID]ACL
)
