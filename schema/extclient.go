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

// UpsertViolations replaces stored violations for this ext client with the
// latest evaluation cycle, then updates severity / cycle metadata on the
// row. Mirrors schema.Node.UpsertViolations - see that method for the
// concurrent-writer rationale (only the previously persisted cycle is
// deleted, so concurrent writers don't erase each other's current rows).
func (e *Extclient) UpsertViolations(ctx context.Context, violations []PostureCheckViolation) error {
	txCtx := db.BeginTx(ctx)
	tx := db.FromContext(txCtx)
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	cycleID := e.PostureCheckLastEvaluationCycleID

	var previousCycleID string
	if err := tx.Model(&Extclient{}).
		Select("posture_check_last_evaluation_cycle_id").
		Where("id = ?", e.ID).
		Scan(&previousCycleID).Error; err != nil {
		return err
	}

	if len(violations) > 0 {
		for i := range violations {
			if violations[i].TenantID == "" {
				violations[i].TenantID = e.TenantID
			}
			if violations[i].NodeID == "" {
				violations[i].NodeID = e.ID
			}
			if violations[i].EvaluationCycleID == "" {
				violations[i].EvaluationCycleID = cycleID
			}
			violations[i].SubjectType = PostureCheckSubjectType_ExtClient
		}
		if err := tx.Model(&PostureCheckViolation{}).Create(&violations).Error; err != nil {
			return err
		}
	}

	if err := tx.Model(&Extclient{}).
		Where("id = ?", e.ID).
		Updates(map[string]interface{}{
			"posture_check_severity":                 e.PostureCheckSeverity,
			"posture_check_last_evaluation_cycle_id": cycleID,
			"posture_check_last_evaluated_at":        e.PostureCheckLastEvaluatedAt,
		}).Error; err != nil {
		return err
	}

	// Drop only the cycle we replaced, same rationale as schema.Node.UpsertViolations.
	if previousCycleID != "" && previousCycleID != cycleID {
		del := tx.Model(&PostureCheckViolation{}).
			Where("node_id = ? AND evaluation_cycle_id = ?", e.ID, previousCycleID)
		if tenantID := scope.ID(ctx); tenantID != "" {
			del = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(del)
		}
		if err := del.Delete(&PostureCheckViolation{}).Error; err != nil {
			return err
		}
	} else if cycleID == "" {
		del := tx.Model(&PostureCheckViolation{}).Where("node_id = ?", e.ID)
		if tenantID := scope.ID(ctx); tenantID != "" {
			del = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(del)
		}
		if err := del.Delete(&PostureCheckViolation{}).Error; err != nil {
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	committed = true
	return nil
}

// ListViolations returns this ext client's violations for its current
// evaluation cycle, falling back to any stored rows if the cycle and
// severity disagree (partial upsert from an older delete-first writer).
// Mirrors schema.Node.ListViolations.
func (e *Extclient) ListViolations(ctx context.Context) ([]PostureCheckViolation, error) {
	var violations []PostureCheckViolation
	query := db.FromContext(ctx).Model(&PostureCheckViolation{}).Where("node_id = ?", e.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(query)
	}

	if e.PostureCheckLastEvaluationCycleID != "" {
		err := query.Where("evaluation_cycle_id = ?", e.PostureCheckLastEvaluationCycleID).Find(&violations).Error
		if err != nil {
			return nil, err
		}
		if len(violations) > 0 || e.PostureCheckSeverity == SeverityUnknown {
			return violations, nil
		}
	}

	// Fallback when severity says violated but the stored cycle has no rows
	// (partial upsert from older delete-first writers).
	fallback := db.FromContext(ctx).Model(&PostureCheckViolation{}).Where("node_id = ?", e.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		fallback = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(fallback)
	}
	err := fallback.Find(&violations).Error
	return violations, err
}

// DeleteViolations removes all stored violations for this ext client.
func (e *Extclient) DeleteViolations(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ?", e.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(query)
	}
	return query.Delete(&PostureCheckViolation{}).Error
}
