package models

type NetworkID string
type RsrcID string
type UserRole string

const (
	HostRsrc           RsrcID = "host"
	RelayRsrc          RsrcID = "relay"
	RemoteAccessGwRsrc RsrcID = "remote_access_gw"
	InetGwRsrc         RsrcID = "inet_gw"
	EgressGwRsrc       RsrcID = "egress"
)

// Pre-Defined User Roles

const (
	SuperAdminRole UserRole = "super_admin"
	AdminRole      UserRole = "admin"
	NetworkAdmin   UserRole = "network_admin"
	NetworkUser    UserRole = "network_user"
)

func (r UserRole) String() string {
	return string(r)
}

type NetworkRsrcPermissions struct {
	Create bool `json:"create"`
	Read   bool `json:"read"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
}

type NetworkAccessControls struct {
	NetworkID                  string                            `json:"network_id"`
	FullAccess                 bool                              `json:"full_access"`
	NetworkRsrcPermissionsList map[RsrcID]NetworkRsrcPermissions `json:"network_permissions_list"`
}

type DashboardAccessControls struct {
	FullAccess          bool                                `json:"full_access"`
	DenyDashboardAccess bool                                `json:"deny_dashboard_access"`
	NetworkLevelAccess  map[NetworkID]NetworkAccessControls `json:"network_access_controls"`
}

type UserPermissionTemplate struct {
	ID            UserRole                `json:"id"`
	Default       bool                    `json:"default"`
	DashBoardAcls DashboardAccessControls `json:"dashboard_access_controls"`
}
