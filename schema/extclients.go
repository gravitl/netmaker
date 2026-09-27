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
	"gorm.io/gorm/clause"
)

const extclientsTable = "extclients_v1"

var ErrExtclientIdentifiersNotProvided = errors.New("extclient identifiers not provided")

type Extclient struct {
	ID       string `gorm:"primaryKey" json:"id"`
	TenantID string `gorm:"default:'';index" json:"tenant_id"`
	// NetworkID has no foreign key constraint, extclients are deleted
	// explicitly on network deletion.
	NetworkID string   `gorm:"not null;index" json:"network_id"`
	Network   *Network `gorm:"foreignKey:NetworkID;constraint:-" json:"network,omitempty"`
	Name      string   `gorm:"not null" json:"name"`

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
	UpdatedAt time.Time `json:"lastmodified"`
}

func (e *Extclient) TableName() string {
	return extclientsTable
}

func (e *Extclient) AddressIPNet4() net.IPNet {
	return net.IPNet{IP: net.ParseIP(e.Address), Mask: net.CIDRMask(32, 32)}
}

func (e *Extclient) AddressIPNet6() net.IPNet {
	return net.IPNet{IP: net.ParseIP(e.Address6), Mask: net.CIDRMask(128, 128)}
}

// baseIdentifierQuery scopes the query to the extclient identified by its ID,
// or by its name in the network identified by NetworkID.
func (e *Extclient) baseIdentifierQuery(ctx context.Context) (*gorm.DB, error) {
	query := db.FromContext(ctx).Model(&Extclient{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", extclientsTable), tenantID)
	}

	if e.ID != "" {
		return query.Where(fmt.Sprintf("%s.id = ?", extclientsTable), e.ID), nil
	}

	if e.Name == "" || e.NetworkID == "" {
		return nil, ErrExtclientIdentifiersNotProvided
	}

	return query.Where(
		fmt.Sprintf("%s.network_id = ? AND %s.name = ?", extclientsTable, extclientsTable),
		e.NetworkID, e.Name,
	), nil
}

// networkQuery scopes the query to the extclients of the network identified
// by NetworkID, or by Network.Name within the tenant in the context. The
// latter joins the network, and populates the Network of the results.
func (e *Extclient) networkQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&Extclient{})
	if tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", extclientsTable), tenantID)
	}

	if e.NetworkID != "" {
		return query.Where(fmt.Sprintf("%s.network_id = ?", extclientsTable), e.NetworkID), nil
	}

	// network names are only unique within a tenant.
	if e.Network == nil || e.Network.Name == "" || tenantID == "" {
		return nil, ErrExtclientIdentifiersNotProvided
	}

	condition := db.FromContext(ctx).Session(&gorm.Session{NewDB: true}).Where(&Network{Name: e.Network.Name})
	return query.InnerJoins("Network", condition), nil
}

func (e *Extclient) Create(ctx context.Context) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&Extclient{}).Omit(clause.Associations).Create(e).Error
}

// Get fetches the extclient by its ID, by its name in the network identified
// by NetworkID, or by its name in the network identified by Network.Name.
func (e *Extclient) Get(ctx context.Context) error {
	var query *gorm.DB
	var err error
	if e.ID == "" && e.NetworkID == "" {
		if e.Name == "" {
			return ErrExtclientIdentifiersNotProvided
		}

		query, err = e.networkQuery(ctx)
		if err == nil {
			query = query.Where(fmt.Sprintf("%s.name = ?", extclientsTable), e.Name)
		}
	} else {
		query, err = e.baseIdentifierQuery(ctx)
	}
	if err != nil {
		return err
	}

	var extclient Extclient
	err = query.First(&extclient).Error
	if err != nil {
		return err
	}

	*e = extclient
	return nil
}

// Update overwrites all the fields of the extclient identified by its ID,
// except the tenant, the creation time and the posture check fields, which
// are owned by UpsertViolations. It returns gorm.ErrRecordNotFound if the
// extclient does not exist.
func (e *Extclient) Update(ctx context.Context) error {
	if e.ID == "" {
		return ErrExtclientIdentifiersNotProvided
	}

	result := db.FromContext(ctx).Model(&Extclient{}).
		Where(fmt.Sprintf("%s.id = ?", extclientsTable), e.ID).
		Select("*").
		Omit(
			"id", "tenant_id", "created_at",
			"posture_check_severity", "posture_check_last_evaluation_cycle_id", "posture_check_last_evaluated_at",
			clause.Associations,
		).
		Updates(e)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete deletes the extclient. It returns gorm.ErrRecordNotFound if the
// extclient does not exist.
func (e *Extclient) Delete(ctx context.Context) error {
	query, err := e.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	result := query.Delete(&Extclient{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListByNetwork lists the extclients of the network identified by NetworkID,
// or by Network.Name.
func (e *Extclient) ListByNetwork(ctx context.Context, options ...dbtypes.Option) ([]Extclient, error) {
	query, err := e.networkQuery(ctx)
	if err != nil {
		return nil, err
	}

	for _, opt := range options {
		query = opt(query)
	}

	var extclients []Extclient
	err = query.Find(&extclients).Error
	return extclients, err
}

// CountByNetwork counts the extclients of the network identified by
// NetworkID, or by Network.Name.
func (e *Extclient) CountByNetwork(ctx context.Context, options ...dbtypes.Option) (int, error) {
	query, err := e.networkQuery(ctx)
	if err != nil {
		return 0, err
	}

	for _, opt := range options {
		query = opt(query)
	}

	var count int64
	err = query.Count(&count).Error
	return int(count), err
}

// DeleteByNetwork deletes the extclients of the network identified by
// NetworkID.
func (e *Extclient) DeleteByNetwork(ctx context.Context) error {
	if e.NetworkID == "" {
		return ErrExtclientIdentifiersNotProvided
	}

	query := db.FromContext(ctx).Where(fmt.Sprintf("%s.network_id = ?", extclientsTable), e.NetworkID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", extclientsTable), tenantID)
	}

	return query.Delete(&Extclient{}).Error
}

func (e *Extclient) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Extclient, error) {
	query := db.FromContext(ctx).Model(&Extclient{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extclientsTable), tenantID))
	}

	for _, opt := range options {
		query = opt(query)
	}

	var extclients []Extclient
	err := query.Find(&extclients).Error
	return extclients, err
}

func (e *Extclient) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	query := db.FromContext(ctx).Model(&Extclient{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extclientsTable), tenantID))
	}

	for _, opt := range options {
		query = opt(query)
	}

	var count int64
	err := query.Count(&count).Error
	return int(count), err
}

func (e *Extclient) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ?", extclientsTable), tenantID).Error
	}

	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", extclientsTable)).Error
}

// UpsertViolations replaces stored violations for this extclient with the
// latest evaluation cycle, then updates severity / cycle metadata on the
// extclient row. Mirrors Node.UpsertViolations, see that method for the
// concurrent writer rationale.
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
			if violations[i].SubjectID == "" {
				violations[i].SubjectID = e.ID
			}
			if violations[i].EvaluationCycleID == "" {
				violations[i].EvaluationCycleID = cycleID
			}
			violations[i].SubjectType = PostureCheckSubjectTypeExtclient
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

	// Drop only the cycle we replaced, same rationale as Node.UpsertViolations.
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
		// Clean extclient with no new cycle — purge any leftover rows for it.
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

// ListViolations returns this extclient's violations for its current
// evaluation cycle. Mirrors Node.ListViolations.
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

func (e *Extclient) DeleteViolations(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ?", e.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(query)
	}
	return query.Delete(&PostureCheckViolation{}).Error
}
