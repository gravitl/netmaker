package models

import "github.com/gravitl/netmaker/schema"

type TagID = schema.TagID
type Tag = schema.Tag

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
