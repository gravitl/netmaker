package schema

import (
	"context"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type NetworkACL struct {
	ID     string `gorm:"primaryKey"`
	Access datatypes.JSONType[map[string]map[string]byte]
}

func (n *NetworkACL) TableName() string {
	return "network_acls_v1"
}

func (n *NetworkACL) Create(ctx context.Context) error {
	if n.Access.Data() == nil {
		n.Access = datatypes.NewJSONType(map[string]map[string]byte{})
	}

	return db.FromContext(ctx).Model(&NetworkACL{}).Create(n).Error
}

func (n *NetworkACL) Get(ctx context.Context) error {
	err := db.FromContext(ctx).Model(n).First(n).Error
	if err != nil {
		return err
	}

	if n.Access.Data() == nil {
		n.Access = datatypes.NewJSONType(map[string]map[string]byte{})
	}

	return nil
}

func (n *NetworkACL) Update(ctx context.Context) error {
	if n.Access.Data() == nil {
		n.Access = datatypes.NewJSONType(map[string]map[string]byte{})
	}

	return db.FromContext(ctx).Model(n).Updates(n).Error
}

func (n *NetworkACL) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Delete(n).Error
}
