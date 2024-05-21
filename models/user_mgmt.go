package models

type NetworkID string
type RsrcID string

const (
	HostRsrc           RsrcID = "host"
	RelayRsrc          RsrcID = "relay"
	RemoteAccessGwRsrc RsrcID = "remote_access_gw"
	InetGwRsrc         RsrcID = "inet_gw"
	EgressGwRsrc       RsrcID = "egress"
)

type NetworkRsrcPermissions struct {
	All    bool `json:"all"`
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
	FullAccess         bool                                `json:"full_access"`
	NetworkLevelAccess map[NetworkID]NetworkAccessControls `json:"network_access_controls"`
}

type UserPermissionTemplate struct {
	ID            string                  `json:"id"`
	DashBoardAcls DashboardAccessControls `json:"dashboard_access_controls"`
}
