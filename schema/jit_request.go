package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
)

const jitRequestTable = "jit_requests"

type JITRequest struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	NetworkID     string    `gorm:"network_id" json:"network_id"`
	UserID        string    `gorm:"user_id" json:"user_id"`
	UserName      string    `gorm:"user_name" json:"user_name"`
	Reason        string    `gorm:"reason" json:"reason"`
	Status        string    `gorm:"status" json:"status"` // pending, approved, denied, expired
	RevokedAt     time.Time `gorm:"revoked_at" json:"revoked_at"`
	RequestedAt   time.Time `gorm:"requested_at" json:"requested_at"`
	ApprovedAt    time.Time `gorm:"approved_at" json:"approved_at,omitempty"`
	ApprovedBy    string    `gorm:"approved_by" json:"approved_by,omitempty"`
	DurationHours int       `gorm:"duration_hours" json:"duration_hours,omitempty"`
	ExpiresAt     time.Time `gorm:"expires_at" json:"expires_at,omitempty"`
}

func (r *JITRequest) Table() string {
	return jitRequestTable
}

func (r *JITRequest) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(r.Table()).Where("id = ?", r.ID).First(&r).Error
}

func (r *JITRequest) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(r.Table()).Create(&r).Error
}

func (r *JITRequest) Update(ctx context.Context) error {
	return db.FromContext(ctx).Table(r.Table()).Where("id = ?", r.ID).Updates(&r).Error
}

func (r *JITRequest) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Table(r.Table()).Where("id = ?", r.ID).Delete(&r).Error
}

func (r *JITRequest) ListByNetwork(ctx context.Context) ([]JITRequest, error) {
	var requests []JITRequest
	err := db.FromContext(ctx).Table(r.Table()).Where("network_id = ?", r.NetworkID).Order("requested_at DESC").Find(&requests).Error
	return requests, err
}

func (r *JITRequest) ListByUserAndNetwork(ctx context.Context) ([]JITRequest, error) {
	var requests []JITRequest
	err := db.FromContext(ctx).Table(r.Table()).Where("network_id = ? AND user_id = ?", r.NetworkID, r.UserID).Find(&requests).Error
	return requests, err
}

func (r *JITRequest) ListPendingByNetwork(ctx context.Context) ([]JITRequest, error) {
	var requests []JITRequest
	err := db.FromContext(ctx).Table(r.Table()).Where("network_id = ? AND status = ?", r.NetworkID, "pending").Find(&requests).Error
	return requests, err
}

func (r *JITRequest) ListByStatusAndNetwork(ctx context.Context, status string) ([]JITRequest, error) {
	var requests []JITRequest
	err := db.FromContext(ctx).Table(r.Table()).Where("network_id = ? AND status = ?", r.NetworkID, status).Order("requested_at DESC").Find(&requests).Error
	return requests, err
}

func (r *JITRequest) CountByNetwork(ctx context.Context) (int64, error) {
	var count int64
	err := db.FromContext(ctx).Table(r.Table()).Where("network_id = ?", r.NetworkID).Count(&count).Error
	return count, err
}

func (r *JITRequest) CountByStatusAndNetwork(ctx context.Context, status string) (int64, error) {
	var count int64
	err := db.FromContext(ctx).Table(r.Table()).Where("network_id = ? AND status = ?", r.NetworkID, status).Count(&count).Error
	return count, err
}
