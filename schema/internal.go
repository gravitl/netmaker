package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

type Internal struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}

func (i *Internal) TableName() string {
	return "__internal__"
}

func (i *Internal) Set(ctx context.Context) error {
	return db.FromContext(ctx).Save(i).Error
}

func (i *Internal) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Internal{}).
		Where("key = ?", i.Key).
		First(i).
		Error
}
