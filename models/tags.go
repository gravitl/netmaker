package models

import (
	"github.com/gravitl/netmaker/schema"
)

type CreateTagReq struct {
	TagName     string           `json:"tag_name"`
	Network     schema.NetworkID `json:"network"`
	ColorCode   string           `json:"color_code"`
	TaggedNodes []ApiNode        `json:"tagged_nodes"`
}

type TagListResp struct {
	schema.Tag
	UsedByCnt   int       `json:"used_by_count"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}

type TagListRespNodes struct {
	schema.Tag
	UsedByCnt   int       `json:"used_by_count"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}

type UpdateTagReq struct {
	schema.Tag
	NewName     string    `json:"new_name"`
	ColorCode   string    `json:"color_code"`
	TaggedNodes []ApiNode `json:"tagged_nodes"`
}
