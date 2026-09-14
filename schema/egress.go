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

const egressTable = "egresses"

type EgressNATMode string

const (
	DisabledNAT EgressNATMode = "disabled"
	VirtualNAT  EgressNATMode = "virtual_nat"
	DirectNAT   EgressNATMode = "direct_nat"
)

// EgressType classifies an egress resource's routing purpose.
type EgressType string

const (
	EgressTypeCIDR     EgressType = "cidr"
	EgressTypeDomain   EgressType = "domain"
	EgressTypeApp      EgressType = "app"
	EgressTypeInternet EgressType = "internet"
)

type Egress struct {
	ID           string            `gorm:"primaryKey" json:"id"`
	TenantID     string            `gorm:"default:'';index" json:"tenant_id"`
	Name         string            `gorm:"name" json:"name"`
	Network      string            `gorm:"network" json:"network"`
	Description  string            `gorm:"description" json:"description"`
	Type         EgressType        `gorm:"column:egress_type;default:cidr;index" json:"type"`
	Nodes        datatypes.JSONMap `gorm:"nodes" json:"nodes"`
	Tags         datatypes.JSONMap `gorm:"tags" json:"tags"`
	Range        string            `gorm:"range" json:"range"`
	Mode         EgressNATMode     `gorm:"mode;default:direct_nat" json:"mode"`
	VirtualRange string            `gorm:"virtual_range" json:"virtual_range"`
	// Domains is the user-configured hostname list (exact or *.suffix).
	Domains datatypes.JSONSlice[string] `gorm:"domains" json:"domains"`
	// DomainAnsByDomain maps each configured domain to its resolved CIDRs.
	DomainAnsByDomain datatypes.JSONMap `gorm:"domain_ans_by_domain" json:"domain_ans_by_domain"`
	Nat               bool              `gorm:"nat" json:"nat"`
	// BypassEgressRoutes (internet egress only): when true, authorized specific
	// Netmaker Egress CIDRs are retained as direct peers and take precedence over
	// this exit via longest-prefix match. When false, IGW clients stay fully
	// relayed through the exit (legacy hairpin behavior).
	BypassEgressRoutes bool `gorm:"column:bypass_egress_routes;default:true" json:"bypass_egress_routes"`
	// PresetID is the catalog id when this egress was created from a preset (empty if custom).
	PresetID  string    `gorm:"preset_id" json:"preset_id"`
	Status    bool      `gorm:"status" json:"status"`
	CreatedBy string    `gorm:"created_by" json:"created_by"`
	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"updated_at" json:"updated_at"`
}

func (e *Egress) Table() string {
	return egressTable
}

func (e *Egress) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).First(e).Error
}

func (e *Egress) Update(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(&e).Error
}

func (e *Egress) UpdateNatStatus(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"nat": e.Nat,
	}).Error
}

func (e *Egress) UpdateEgressStatus(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"status": e.Status,
	}).Error
}

func (e *Egress) ResetDomain(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"domain":               "",
		"domains":              datatypes.JSONSlice[string]([]string{}),
		"domain_ans_by_domain": datatypes.JSONMap{},
	}).Error
}

func (e *Egress) ResetRange(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"range": "",
	}).Error
}

func (e *Egress) ResetVirtualRange(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"virtual_range": "",
	}).Error
}
func (e *Egress) ResetMode(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"mode": "",
	}).Error
}

func (e *Egress) DoesEgressRouteExists(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("range = ?", e.Range).First(&e).Error
}

func (e *Egress) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Create(&e).Error
}

func (e *Egress) ListAll(ctx context.Context) (egs []Egress, err error) {
	query := db.FromContext(ctx).Table(e.Table())
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", egressTable), tenantID)(query)
	}
	err = query.Find(&egs).Error
	return
}

func (e *Egress) ListByNetwork(ctx context.Context) (egs []Egress, err error) {
	query := db.FromContext(ctx).Table(e.Table()).Where("network = ?", e.Network)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", egressTable), tenantID)(query)
	}
	err = query.Find(&egs).Error
	return
}

func (e *Egress) Count(ctx context.Context) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&Egress{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", egressTable), tenantID)(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}

func (e *Egress) Delete(ctx context.Context, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Egress{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", egressTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	if e.ID != "" {
		query = query.Where("id = ?", e.ID)
	}
	return query.Delete(&Egress{}).Error
}
