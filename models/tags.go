package models

import "time"

type TagID string

func (id TagID) String() string {
	return string(id)
}

type Tag struct {
	ID        TagID     `json:"id"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTagReq struct {
	ID          TagID    `json:"id"`
	TaggedHosts []string `json:"tagged_hosts"`
}

type TagListResp struct {
	Tag
	UsedByCnt   int    `json:"used_by_count"`
	TaggedHosts []Host `json:"tagged_hosts"`
}

type UpdateTagReq struct {
	Tag
	NewID       TagID    `json:"new_id"`
	TaggedHosts []string `json:"tagged_hosts"`
}
