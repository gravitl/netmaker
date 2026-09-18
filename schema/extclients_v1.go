package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

type ExtClientV1 struct {
	ID        string `gorm:"primaryKey" json:"id"`
	TenantID  string `gorm:"default:'';uniqueIndex:idx_extclients_v1_tenant_network_name" json:"tenant_id"`
	NetworkID string `gorm:"uniqueIndex:idx_extclients_v1_tenant_network_name" json:"network_id"`
	Name      string `gorm:"uniqueIndex:idx_extclients_v1_tenant_network_name" json:"name"`

	PrivateKey string `json:"privatekey"`
	PublicKey  string `json:"publickey"`
	DNS        string `json:"dns"`
	Address    string `json:"address"`
	Address6   string `json:"address6"`

	IngressGatewayID       string `json:"ingressgatewayid"`
	IngressGatewayEndpoint string `json:"ingressgatewayendpoint"`

	AllowedIPs      datatypes.JSONSlice[string] `json:"allowed_ips"`
	ExtraAllowedIPs datatypes.JSONSlice[string] `json:"extraallowedips"`

	PostUp   string `json:"postup"`
	PostDown string `json:"postdown"`

	// SelectedInternetEgressID is the internet egress this config file uses for full-tunnel exit (empty = none).
	SelectedInternetEgressID string `json:"selected_internet_egress_id"`
	Enabled                  bool   `json:"enabled"`
	OwnerID                  string `json:"ownerid"`

	Status                            NodeStatus `json:"status"`
	PostureCheckSeverity              Severity   `json:"posture_check_severity"`
	PostureCheckLastEvaluationCycleID string     `json:"posture_check_last_evaluation_cycle_id"`
	PostureCheckLastEvaluatedAt       time.Time  `json:"posture_check_last_evaluated_at"`

	// The fields below only matter for clients that aren't backed by a Node
	// (mobile apps and other RAC clients) - they don't get these from a Host
	// the way a Node would.
	DeniedACLs           datatypes.JSONType[map[string]struct{}] `json:"deniednodeacls"`
	RemoteAccessClientID string                                  `json:"remote_access_client_id"`
	Tags                 datatypes.JSONType[map[TagID]struct{}]  `json:"tags"`
	OS                   string                                  `json:"os"`
	OSFamily             string                                  `json:"os_family"`
	OSVersion            string                                  `json:"os_version"`
	KernelVersion        string                                  `json:"kernel_version"`
	ClientVersion        string                                  `json:"client_version"`
	DeviceID             string                                  `json:"device_id"`
	DeviceName           string                                  `json:"device_name"`
	PublicEndpoint       string                                  `json:"public_endpoint"`
	Country              string                                  `json:"country"`
	Location             string                                  `json:"location"`
	JITExpiresAt         *time.Time                              `json:"jit_expires_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const extClientsV1Table = "extclients_v1"

func (*ExtClientV1) TableName() string { return extClientsV1Table }

func (e *ExtClientV1) Create(ctx context.Context) error {
	e.TenantID = scope.ID(ctx)
	return db.FromContext(ctx).Model(&ExtClientV1{}).Create(e).Error
}

func (e *ExtClientV1) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&ExtClientV1{}).Where("id = ?", e.ID).First(e).Error
}

func (e *ExtClientV1) GetByName(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&ExtClientV1{}).Where("name = ? AND network_id = ?", e.Name, e.NetworkID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extClientsV1Table), tenantID)(query)
	}
	return query.First(e).Error
}

func (e *ExtClientV1) Update(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *ExtClientV1) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&ExtClientV1{}).Where("id = ?", e.ID).Delete(e).Error
}

func (*ExtClientV1) ListAll(ctx context.Context, options ...dbtypes.Option) ([]ExtClientV1, error) {
	var clients []ExtClientV1
	query := db.FromContext(ctx).Model(&ExtClientV1{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extClientsV1Table), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Find(&clients).Error
	return clients, err
}

func (*ExtClientV1) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", extClientsV1Table), tenantID).Delete(&ExtClientV1{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", extClientsV1Table)).Error
}

func (*ExtClientV1) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&ExtClientV1{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extClientsV1Table), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
