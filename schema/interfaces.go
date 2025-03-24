package schema

import (
	"context"
	"errors"
	"github.com/gravitl/netmaker/db"
	"gorm.io/gorm"
)

type Interface struct {
	HostID        string `gorm:"primaryKey"`
	Name          string `gorm:"primaryKey"`
	Address       string
	AddressString string
}

func (i *Interface) TableName() string {
	return "interfaces"
}

func (i *Interface) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(i.TableName()).Create(i).Error
}

func (i *Interface) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(i.TableName()).
		Where("host_id = ? AND name = ?", i.HostID, i.Name).
		First(i).Error
}

func (i *Interface) ListAllByHost(ctx context.Context) ([]Interface, error) {
	var interfaces []Interface
	err := db.FromContext(ctx).Table(i.TableName()).
		Where("host_id = ?", i.HostID).
		Find(&interfaces).Error
	return interfaces, err
}

func (i *Interface) Upsert(ctx context.Context) error {
	var existingInterface Interface
	err := db.FromContext(ctx).Table(i.TableName()).
		Where("host_id = ? AND name = ?", i.HostID, i.Name).
		First(&existingInterface).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return i.Create(ctx)
		}

		return err
	}

	return db.FromContext(ctx).Table(i.TableName()).
		Where("host_id = ? AND name = ?", i.HostID, i.Name).
		Updates(i).Error
}

func (i *Interface) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Table(i.TableName()).
		Where("host_id = ? AND name = ?", i.HostID, i.Name).
		Delete(i).Error
}
