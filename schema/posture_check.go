package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
)

type Attribute string
type Values string

const (
	OS             Attribute = "os"
	OSVersion      Attribute = "os_version"
	OSFamily       Attribute = "os_family"
	KernelVersion  Attribute = "kernel_version"
	AutoUpdate     Attribute = "auto_update"
	ClientVersion  Attribute = "client_version"
	ClientLocation Attribute = "client_location"
)

var PostureCheckAttrs = []Attribute{
	ClientLocation,
	ClientVersion,
	OS,
	OSVersion,
	OSFamily,
	KernelVersion,
	AutoUpdate,
}

var PostureCheckAttrValuesMap = map[Attribute]map[string]struct{}{
	ClientLocation: {
		"any_valid_iso_country_codes": {},
	},
	ClientVersion: {
		"any_valid_semantic_version": {},
	},
	OS: {
		"linux":   {},
		"darwin":  {},
		"windows": {},
		"ios":     {},
		"android": {},
	},
	OSVersion: {
		"any_valid_semantic_version": {},
	},
	OSFamily: {
		"linux-debian": {},
		"linux-redhat": {},
		"linux-suse":   {},
		"linux-arch":   {},
		"linux-gentoo": {},
		"linux-other":  {},
		"darwin":       {},
		"windows":      {},
		"ios":          {},
		"android":      {},
	},
	KernelVersion: {
		"any_valid_semantic_version": {},
	},
	AutoUpdate: {
		"true":  {},
		"false": {},
	},
}

var PostureCheckAttrValues = map[Attribute][]string{
	ClientLocation: {"any_valid_iso_country_codes"},
	ClientVersion:  {"any_valid_semantic_version"},
	OS:             {"linux", "darwin", "windows", "ios", "android"},
	OSVersion:      {"any_valid_semantic_version"},
	OSFamily:       {"linux-debian", "linux-redhat", "linux-suse", "linux-arch", "linux-gentoo", "linux-other", "darwin", "windows", "ios", "android"},
	KernelVersion:  {"any_valid_semantic_version"},
	AutoUpdate:     {"true", "false"},
}

type PostureCheck struct {
	ID          string                      `gorm:"primaryKey" json:"id"`
	Name        string                      `gorm:"name" json:"name"`
	NetworkID   string                      `gorm:"network_id" json:"network_id"`
	Description string                      `gorm:"description" json:"description"`
	Attribute   Attribute                   `gorm:"attribute" json:"attribute"`
	Values      datatypes.JSONSlice[string] `gorm:"values" json:"values"`
	Severity    models.Severity             `gorm:"severity" json:"severity"`
	Tags        datatypes.JSONMap           `gorm:"tags" json:"tags"`
	Status      bool                        `gorm:"status" json:"status"`
	CreatedBy   string                      `gorm:"created_by" json:"created_by"`
	CreatedAt   time.Time                   `gorm:"created_at" json:"created_at"`
	UpdatedAt   time.Time                   `gorm:"updated_at" json:"updated_at"`
}

func (p *PostureCheck) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PostureCheck{}).First(&p).Where("id = ?", p.ID).Error
}

func (p *PostureCheck) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PostureCheck{}).Where("id = ?", p.ID).Updates(&p).Error
}

func (p *PostureCheck) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PostureCheck{}).Create(&p).Error
}

func (p *PostureCheck) ListByNetwork(ctx context.Context) (pcli []PostureCheck, err error) {
	err = db.FromContext(ctx).Model(&PostureCheck{}).Where("network_id = ?", p.NetworkID).Find(&pcli).Error
	return
}

func (p *PostureCheck) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PostureCheck{}).Where("id = ?", p.ID).Delete(&p).Error
}

func (p *PostureCheck) UpdateStatus(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PostureCheck{}).Where("id = ?", p.ID).Updates(map[string]any{
		"status": p.Status,
	}).Error
}
