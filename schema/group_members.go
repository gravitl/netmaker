package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

type GroupMember struct {
	GroupID string `gorm:"primaryKey"`
	UserID  string `gorm:"primaryKey"`
}

func (g *GroupMember) TableName() string {
	return "group_members_v1"
}

func (g *GroupMember) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&GroupMember{}).Create(g).Error
}

func (g *GroupMember) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM group_members_v1 WHERE group_id = ? AND user_id = ?)",
		g.GroupID,
		g.UserID,
	).Scan(&exists).Error
	return exists, err
}

func (g *GroupMember) ListAllMembers(ctx context.Context) ([]GroupMember, error) {
	var members []GroupMember
	err := db.FromContext(ctx).Model(&GroupMember{}).
		Where("group_id = ?", g.GroupID).
		Find(&members).
		Error
	return members, err
}

func (g *GroupMember) ListAllGroups(ctx context.Context) ([]GroupMember, error) {
	var members []GroupMember
	err := db.FromContext(ctx).Model(&GroupMember{}).
		Where("user_id = ?", g.UserID).
		Find(&members).
		Error
	return members, err
}

func (g *GroupMember) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&GroupMember{}).Delete(g).Error
}
