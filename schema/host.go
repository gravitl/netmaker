package schema

import (
	"context"
	"fmt"
	"github.com/gravitl/netmaker/db"
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
	Interfaces          []Interface `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	DefaultInterface    string
	Interface           string
	PublicKey           string
	TrafficKeyPublic    string
}

func (h *Host) Create(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		if h.DefaultInterface != "" {
			found := false
			for _, iface := range h.Interfaces {
				if iface.Name == h.DefaultInterface {
					found = true
				}
			}

			if !found {
				return fmt.Errorf(
					"constraint violation: default interface '%s' not found in interfaces list",
					h.DefaultInterface,
				)
			}
		}

		return tx.Model(&Host{}).Create(h).Error
	})
}

func (h *Host) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).
		Joins("LEFT JOIN interfaces ON interfaces.host_id = hosts.id").
		Where("hosts.id = ?", h.ID).
		First(h).
		Error
}

func (h *Host) ListAll(ctx context.Context) ([]Host, error) {
	var hosts []Host
	err := db.FromContext(ctx).Model(&Host{}).
		Joins("LEFT JOIN interfaces ON interfaces.host_id = hosts.id").
		Find(&hosts).
		Error
	return hosts, err
}

func (h *Host) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Host{}).Count(&count).Error
	return int(count), err
}

func (h *Host) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		if h.DefaultInterface != "" {
			found := false
			for _, iface := range h.Interfaces {
				if iface.Name == h.DefaultInterface {
					found = true
				}
			}

			if !found {
				return fmt.Errorf(
					"constraint violation: default interface '%s' not found in interfaces list",
					h.DefaultInterface,
				)
			}
		}

		err := tx.Model(&Host{}).Updates(&h).Error
		if err != nil {
			return err
		}

		return tx.Model(&Host{}).Association("Interfaces").Replace(h.Interfaces)
	})
}

func (h *Host) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).Where("id = ?", h.ID).Delete(h).Error
}
