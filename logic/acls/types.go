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
	// NodeID - the node id of a given node
	NodeID string

	// NetworkID - the networkID of a given network
	NetworkID string

	// NodeACL - the ACL of other nodes in a NetworkACL for a single unique node
	NodeACL map[NodeID]byte

	// NetworkACL - the total list of all node's ACL in a given network
	NetworkACL map[NodeID]NodeACL

	// ACLJson - the string representation in JSON of an ACL Node or Network
	ACLJson string
)
