package schema

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Extclient struct {
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

const extclientTable = "extclients_v1"

func (*Extclient) TableName() string { return extclientTable }

func (e *Extclient) AddressIPNet4() net.IPNet {
	return net.IPNet{IP: net.ParseIP(e.Address), Mask: net.CIDRMask(32, 32)}
}

func (e *Extclient) AddressIPNet6() net.IPNet {
	return net.IPNet{IP: net.ParseIP(e.Address6), Mask: net.CIDRMask(128, 128)}
}

func (e *Extclient) Create(ctx context.Context) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	e.TenantID = scope.ID(ctx)
	return db.FromContext(ctx).Model(&Extclient{}).Create(e).Error
}

func (e *Extclient) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Extclient{}).Where("id = ?", e.ID).First(e).Error
}

var (
	ErrExtClientNameLookupRequiresNetworkID = errors.New("name lookup requires network_id")
	ErrExtClientNameLookupRequiresTenant    = errors.New("name lookup requires a tenant scope")
)

func (e *Extclient) GetByName(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return ErrExtClientNameLookupRequiresTenant
	}
	if e.NetworkID == "" {
		return ErrExtClientNameLookupRequiresNetworkID
	}
	return db.FromContext(ctx).Model(&Extclient{}).
		Where("name = ? AND network_id = ? AND tenant_id = ?", e.Name, e.NetworkID, tenantID).
		First(e).Error
}

func (e *Extclient) Update(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *Extclient) Upsert(ctx context.Context) error {
	existing := &Extclient{Name: e.Name, NetworkID: e.NetworkID}
	err := existing.GetByName(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return e.Create(ctx)
	}

	e.ID = existing.ID
	e.TenantID = existing.TenantID
	e.CreatedAt = existing.CreatedAt
	return e.Update(ctx)
}

func (e *Extclient) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Extclient{}).Where("id = ?", e.ID).Delete(e).Error
}

func (e *Extclient) DeleteByName(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return ErrExtClientNameLookupRequiresTenant
	}
	if e.NetworkID == "" {
		return ErrExtClientNameLookupRequiresNetworkID
	}
	query := db.FromContext(ctx).Where("name = ? AND network_id = ?", e.Name, e.NetworkID)
	query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extclientTable), tenantID)(query)
	return query.Delete(&Extclient{}).Error
}

func (*Extclient) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Extclient, error) {
	var clients []Extclient
	query := db.FromContext(ctx).Model(&Extclient{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extclientTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Find(&clients).Error
	return clients, err
}

func (*Extclient) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", extclientTable), tenantID).Delete(&Extclient{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", extclientTable)).Error
}

func (*Extclient) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&Extclient{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extclientTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
