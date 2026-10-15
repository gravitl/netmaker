package models

import (
	"time"

	"github.com/gravitl/netmaker/schema"
)

type TagID = schema.TagID

// Tag - tag of the nodes and ext clients of a network
type Tag struct {
	// ID is <network name>.<tag name>, which the tag is referred to by.
	ID        TagID            `json:"id"`
	TagName   string           `json:"tag_name"`
	Network   schema.NetworkID `json:"network"`
	ColorCode string           `json:"color_code"`
	CreatedBy string           `json:"created_by"`
	CreatedAt time.Time        `json:"created_at"`
}

const (
	OldRemoteAccessTagName = schema.OldRemoteAccessTagName
	GwTagName              = schema.GwTagName
)

type CreateTagReq struct {
	TagName     string           `json:"tag_name"`
	Network     schema.NetworkID `json:"network"`
	ColorCode   string           `json:"color_code"`
	TaggedNodes []ApiNode        `json:"tagged_nodes"`
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
