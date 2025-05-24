package models

type Action string

const (
	Create            Action = "CREATE"
	Update            Action = "UPDATE"
	Delete            Action = "DELETE"
	DeleteAll         Action = "DELETE_ALL"
	Login             Action = "LOGIN"
	LogOut            Action = "LOGOUT"
	Connect           Action = "CONNECT"
	Sync              Action = "SYNC"
	RefreshKey        Action = "REFRESH_KEY"
	RefreshAllKeys    Action = "REFRESH_ALL_KEYS"
	SyncAll           Action = "SYNC_ALL"
	UpgradeAll        Action = "UPGRADE_ALL"
	Disconnect        Action = "DISCONNECT"
	JoinHostToNet     Action = "JOIN_HOST_TO_NETWORK"
	RemoveHostFromNet Action = "REMOVE_HOST_FROM_NETWORK"
)

type SubjectType string

const (
	UserSub            SubjectType = "USER"
	UserAccessTokenSub SubjectType = "USER_ACCESS_TOKEN"
	DeviceSub          SubjectType = "DEVICE"
	NodeSub            SubjectType = "NODE"
	GatewaySub         SubjectType = "GATEWAY"
	SettingSub         SubjectType = "SETTING"
	AclSub             SubjectType = "ACL"
	TagSub             SubjectType = "TAG"
	UserRoleSub        SubjectType = "USER_ROLE"
	UserGroupSub       SubjectType = "USER_GROUP"
	UserInviteSub      SubjectType = "USER_INVITE"
	PendingUserSub     SubjectType = "PENDING_USER"
	EgressSub          SubjectType = "EGRESS"
	NetworkSub         SubjectType = "NETWORK"
	DashboardSub       SubjectType = "DASHBOARD"
	EnrollmentKeySub   SubjectType = "ENROLLMENT_KEY"
	ClientAppSub       SubjectType = "CLIENT-APP"
)

func (sub SubjectType) String() string {
	return string(sub)
}

type Origin string

const (
	Dashboard Origin = "DASHBOARD"
	Api       Origin = "API"
	NMCTL     Origin = "NMCTL"
	ClientApp Origin = "CLIENT-APP"
)

type Subject struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type SubjectType `json:"subject_type"`
	Info interface{} `json:"info"`
}

type Diff struct {
	Old interface{}
	New interface{}
}

type Event struct {
	Action      Action
	Source      Subject
	Origin      Origin
	Target      Subject
	TriggeredBy string
	NetworkID   NetworkID
	Diff        Diff
}
