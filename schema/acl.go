package schema

import (
	"context"
	"errors"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

type ACL struct {
	ID               string
	NetworkID        string
	Name             string
	MetaData         string
	Default          bool
	Enabled          bool
	PolicyType       string
	ServiceType      string
	AllowedDirection int
	Src              datatypes.JSONSlice[PolicyGroupTag]
	Dst              datatypes.JSONSlice[PolicyGroupTag]
	Protocol         string
	Port             datatypes.JSONSlice[string]
	CreatedBy        string
	CreatedAt        time.Time
}

type PolicyGroupTag struct {
	GroupType string
	Tag       string
}

func (a *ACL) TableName() string {
	return "acls_v1"
}

func (a *ACL) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&ACL{}).Create(a).Error
}

func (a *ACL) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(a).
		Where("id = ?", a.ID).
		First(a).
		Error
}

func (a *ACL) ListAll(ctx context.Context) ([]ACL, error) {
	var acls []ACL
	err := db.FromContext(ctx).Model(&ACL{}).Find(&acls).Error
	return acls, err
}

func (a *ACL) ListNetworkPolicies(ctx context.Context) ([]ACL, error) {
	var networkPolicies []ACL
	err := db.FromContext(ctx).Model(&ACL{}).
		Where("network_id = ?", a.NetworkID).
		Find(&networkPolicies).
		Error
	return networkPolicies, err
}

func (a *ACL) ListEnabledNetworkPolicies(ctx context.Context) ([]ACL, error) {
	var networkPolicies []ACL
	err := db.FromContext(ctx).Model(&ACL{}).
		Where("enabled = ? AND network_id = ?", true, a.NetworkID).
		Find(&networkPolicies).
		Error
	return networkPolicies, err
}

func (a *ACL) ListByPolicyType(ctx context.Context) ([]ACL, error) {
	var networkPolicies []ACL
	err := db.FromContext(ctx).Model(&ACL{}).
		Where("policy_type = ?", a.PolicyType).
		Find(&networkPolicies).
		Error
	return networkPolicies, err
}

func (a *ACL) ListNetworkPoliciesByPolicyType(ctx context.Context) ([]ACL, error) {
	var policies []ACL
	err := db.FromContext(ctx).Model(&ACL{}).
		Where("network_id = ? AND policy_type = ?", a.NetworkID, a.PolicyType).
		Find(&policies).
		Error
	return policies, err
}

func (a *ACL) ListEnabledNetworkPoliciesByPolicyType(ctx context.Context) ([]ACL, error) {
	var policies []ACL
	err := db.FromContext(ctx).Model(&ACL{}).
		Where("enabled = ? AND network_id = ? AND policy_type = ?", true, a.NetworkID, a.PolicyType).
		Find(&policies).
		Error
	return policies, err
}

func (a *ACL) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM acls_v1 WHERE id = ?)",
		a.ID,
	).Scan(&exists).Error
	return exists, err
}

func (a *ACL) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(a).Save(a).Error
}

func (a *ACL) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(a).First(&ACL{
			ID: a.ID,
		}).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Model(&ACL{}).Create(a).Error
			} else {
				return err
			}
		} else {
			return tx.Model(a).Save(a).Error
		}
	})
}

func (a *ACL) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(a).Delete(a).Error
}

func (a *ACL) DeleteDefaultNetworkACLs(ctx context.Context) error {
	return db.FromContext(ctx).Model(&ACL{}).
		Where("default = ? AND network_id = ?", true, a.NetworkID).
		Delete(&ACL{}).
		Error
}
