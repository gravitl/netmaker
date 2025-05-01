package models

type Action string

const (
	Create Action = "CREATE"
	Update Action = "UPDATE"
	Delete Action = "DELETE"
	Login  Action = "LOGIN"
)

type SubjectType string

const (
	UserSub      SubjectType = "USER"
	DeviceSub    SubjectType = "DEVICE"
	NodeSub      SubjectType = "NODE"
	SettingSub   SubjectType = "SETTING"
	AclSub       SubjectType = "ACLs"
	EgressSub    SubjectType = "EGRESS"
	NetworkSub   SubjectType = "NETWORK"
	DashboardSub SubjectType = "DASHBOARD"
	ClientAppSub SubjectType = "CLIENT-APP"
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
}

type Activity struct {
	Action    Action
	Source    Subject
	Origin    Origin
	Target    Subject
	NetworkID NetworkID
}
