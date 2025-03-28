package models

import (
	"fmt"
	"time"
)

type TagID string

const (
	OldRemoteAccessTagName = "remote-access-gws"
	GwTagName              = "gateways"
)

func (id TagID) String() string {
	return string(id)
}

func (t Tag) GetIDFromName() string {
	return fmt.Sprintf("%s.%s", t.Network, t.TagName)
}

type Tag struct {
	ID        TagID     `json:"id"`
	TagName   string    `json:"tag_name"`
	Network   NetworkID `json:"network"`
	ColorCode string    `json:"color_code"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTagReq struct {
	TagName     string    `json:"tag_name"`
	Network     NetworkID `json:"network"`
	ColorCode   string    `json:"color_code"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}

type TagListResp struct {
	Tag
	UsedByCnt   int       `json:"used_by_count"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}

type TagListRespNodes struct {
	Tag
	UsedByCnt   int       `json:"used_by_count"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}

type UpdateTagReq struct {
	Tag
	NewName     string    `json:"new_name"`
	ColorCode   string    `json:"color_code"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}
