package schema

import (
	"context"
	"errors"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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
}

type Interface struct {
	Name    string
	Address string
}

func (h *Host) TableName() string {
	return "hosts_v1"
}

func (h *Host) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).Create(h).Error
}

func (h *Host) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(h).
		Where("id = ?", h.ID).
		First(h).
		Error
}

func (h *Host) GetNodes(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ?", h.ID).
		Find(&nodes).
		Error
	return nodes, err
}

func (h *Host) ListAll(ctx context.Context) ([]Host, error) {
	var hosts []Host
	err := db.FromContext(ctx).Model(&Host{}).Find(&hosts).Error
	return hosts, err
}

func (h *Host) ListZombies(ctx context.Context) ([]Host, error) {
	var hosts []Host
	err := db.FromContext(ctx).Model(&Host{}).
		Where("id <> ? AND mac_address == ?", h.ID, h.MacAddress).
		Find(&hosts).
		Error
	return hosts, err
}

func (h *Host) ListDefaultHosts(ctx context.Context) ([]Host, error) {
	var hosts []Host
	err := db.FromContext(ctx).Model(&Host{}).
		Where("is_default = ?", true).
		Find(&hosts).
		Error
	return hosts, err
}

func (h *Host) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM hosts_v1 WHERE id = ?)",
		h.ID,
	).Scan(&exists).Error
	return exists, err
}

func (h *Host) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Host{}).Count(&count).Error
	return int(count), err
}

func (h *Host) CountNodes(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ?", h.ID).
		Count(&count).
		Error
	return int(count), err
}

func (h *Host) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(h).First(&Host{
			ID: h.ID,
		}).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Model(&Host{}).Create(h).Error
			} else {
				return err
			}
		} else {
			return tx.Model(h).Save(h).Error
		}
	})
}

func (h *Host) UpdateNodesLastCheckIn(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ? AND pending_delete <> ? AND action <> ?", h.ID, true, "delete").
		Update("last_check_in", time.Now()).Error
}

func (h *Host) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(h).Delete(h).Error
}
