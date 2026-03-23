package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type Action string

const (
	Create                               Action = "CREATE"
	Update                               Action = "UPDATE"
	Delete                               Action = "DELETE"
	DeleteAll                            Action = "DELETE_ALL"
	Login                                Action = "LOGIN"
	LogOut                               Action = "LOGOUT"
	Connect                              Action = "CONNECT"
	Sync                                 Action = "SYNC"
	RefreshKey                           Action = "REFRESH_KEY"
	RefreshAllKeys                       Action = "REFRESH_ALL_KEYS"
	SyncAll                              Action = "SYNC_ALL"
	UpgradeAll                           Action = "UPGRADE_ALL"
	Disconnect                           Action = "DISCONNECT"
	JoinHostToNet                        Action = "JOIN_HOST_TO_NETWORK"
	RemoveHostFromNet                    Action = "REMOVE_HOST_FROM_NETWORK"
	EnableMFA                            Action = "ENABLE_MFA"
	DisableMFA                           Action = "DISABLE_MFA"
	EnforceMFA                           Action = "ENFORCE_MFA"
	UnenforceMFA                         Action = "UNENFORCE_MFA"
	EnableBasicAuth                      Action = "ENABLE_BASIC_AUTH"
	DisableBasicAuth                     Action = "DISABLE_BASIC_AUTH"
	EnableTelemetry                      Action = "ENABLE_TELEMETRY"
	DisableTelemetry                     Action = "DISABLE_TELEMETRY"
	UpdateClientSettings                 Action = "UPDATE_CLIENT_SETTINGS"
	UpdateAuthenticationSecuritySettings Action = "UPDATE_AUTHENTICATION_SECURITY_SETTINGS"
	UpdateMonitoringAndDebuggingSettings Action = "UPDATE_MONITORING_AND_DEBUGGING_SETTINGS"
	UpdateSMTPSettings                   Action = "UPDATE_EMAIL_SETTINGS"
	UpdateIDPSettings                    Action = "UPDATE_IDP_SETTINGS"
	EnableFlowLogs                       Action = "ENABLE_FLOW_LOGS"
	DisableFlowLogs                      Action = "DISABLE_FLOW_LOGS"
	GatewayAssign                        Action = "GATEWAY_ASSIGN"
	GatewayUnAssign                      Action = "GATEWAY_UNASSIGN"
	EnableJIT                            Action = "ENABLE_JIT"
	DisableJIT                           Action = "DISABLE_JIT"
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
	NameserverSub      SubjectType = "NAMESERVER"
	PostureCheckSub    SubjectType = "POSTURE_CHECK"
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

type Event struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Action      Action         `gorm:"action" json:"action"`
	Source      datatypes.JSON `gorm:"source" json:"source"`
	Origin      Origin         `gorm:"origin" json:"origin"`
	Target      datatypes.JSON `gorm:"target" json:"target"`
	NetworkID   NetworkID      `gorm:"network_id" json:"network_id"`
	TriggeredBy string         `gorm:"triggered_by" json:"triggered_by"`
	Diff        datatypes.JSON `gorm:"diff" json:"diff"`
	TimeStamp   time.Time      `gorm:"time_stamp" json:"time_stamp"`
}

func (a *Event) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Event{}).First(&a).Where("id = ?", a.ID).Error
}

func (a *Event) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Event{}).Where("id = ?", a.ID).Updates(&a).Error
}

func (a *Event) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Event{}).Create(&a).Error
}

func (a *Event) ListByNetwork(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		// "created_at BETWEEN ? AND ?
		err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ? AND time_stamp BETWEEN ? AND ?",
			a.NetworkID, from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ?", a.NetworkID).Order("time_stamp DESC").Find(&ats).Error

	return
}

func (a *Event) ListByUser(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		err = db.FromContext(ctx).Model(&Event{}).Where("triggered_by = ? AND time_stamp BETWEEN ? AND ?",
			a.TriggeredBy, from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Where("triggered_by = ?", a.TriggeredBy).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) ListByUserAndNetwork(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ? AND triggered_by = ? AND time_stamp BETWEEN ? AND ?",
			a.NetworkID, a.TriggeredBy, from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ? AND triggered_by = ?",
		a.NetworkID, a.TriggeredBy).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) List(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		err = db.FromContext(ctx).Model(&Event{}).Where("time_stamp BETWEEN ? AND ?", from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) DeleteOldEvents(ctx context.Context, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return db.FromContext(ctx).Model(&Event{}).Where("created_at < ?", cutoff).Delete(&Event{}).Error
}
