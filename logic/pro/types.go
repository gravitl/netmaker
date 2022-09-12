package pro

const (
	// == NET ACCESS END == indicates access for system admin (control of netmaker)
	// NET_ADMIN - indicates access for network admin (control of network)
	NET_ADMIN = 0
	// NODE_ACCESS - indicates access for
	NODE_ACCESS = 1
	// CLIENT_ACCESS - indicates access for network user (limited to nodes + ext clients)
	CLIENT_ACCESS = 2
	// NO_ACCESS - indicates user has no access to network
	NO_ACCESS = 3
	// == NET ACCESS END ==
	// DEFAULT_ALLOWED_GROUPS - default user group for all networks
	DEFAULT_ALLOWED_GROUPS = "*"
	// DEFAULT_ALLOWED_USERS - default allowed users for a network
	DEFAULT_ALLOWED_USERS = "*"
	// DB_GROUPS_KEY - represents db groups
	DB_GROUPS_KEY = "netmaker-groups"
)
