package schema

import (
	"context"
	"errors"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

type Host struct {
	ID                  string `gorm:"primaryKey"`
	Name                string
	Password            string
	Version             string
	OS                  string
	DaemonInstalled     bool
	AutoUpdate          bool
	Verbosity           int
	Debug               bool
	IsDocker            bool
	IsK8S               bool
	IsStaticPort        bool
	IsStatic            bool
	IsDefault           bool
	MacAddress          string
	EndpointIP          string
	EndpointIPv6        string
	TurnEndpoint        string
	NatType             string
	ListenPort          int
	WgPublicListenPort  int
	MTU                 int
	FirewallInUse       string
	IPForwarding        bool
	PersistentKeepalive time.Duration
	Interfaces          datatypes.JSONSlice[Interface]
	DefaultInterface    string
	Interface           string
	PublicKey           string
	TrafficKeyPublic    []byte
	Nodes               []Node `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type Interface struct {
	Name    string
	Address string
}

func (h *Host) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).Create(h).Error
}

func (h *Host) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).
		Where("hosts.id = ?", h.ID).
		First(h).
		Error
}

func (h *Host) ListAll(ctx context.Context) ([]Host, error) {
	var hosts []Host
	err := db.FromContext(ctx).Model(&Host{}).Find(&hosts).Error
	return hosts, err
}

func (h *Host) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Host{}).Count(&count).Error
	return int(count), err
}

func (h *Host) Upsert(ctx context.Context) error {
	var err error
	ctx = db.BeginTx(ctx)
	defer func() {
		if err != nil {
			db.FromContext(ctx).Rollback()
		} else {
			db.FromContext(ctx).Commit()
		}
	}()

	err = db.FromContext(ctx).Model(&Host{}).
		Where("id = ?", h.ID).
		First(&Host{}).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = h.Create(ctx)
			return err
		} else {
			return err
		}
	} else {
		err = db.FromContext(ctx).Model(&Host{}).
			Where("id = ?", h.ID).
			Updates(h).
			Error
		return err
	}
}

func (h *Host) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).Where("id = ?", h.ID).Delete(h).Error
}
