package models

type SrcType string
type DstType string

// AllowedTrafficDirection - allowed direction of traffic
type AllowedTrafficDirection int

const (
	// TrafficDirectionUni implies traffic is only allowed in one direction (src --> dst)
	TrafficDirectionUni AllowedTrafficDirection = iota
	// TrafficDirectionBi implies traffic is allowed both direction (src <--> dst )
	TrafficDirectionBi
)

const (
	SrcUser SrcType = "user"
	SrcHost SrcType = "host"

	DstHost DstType = "host"
)

type Acl struct {
	Src              SrcType                 `json:"src_type"`
	Dst              DstType                 `json:"dst_type"`
	AllowedDirection AllowedTrafficDirection `json:"allowed_traffic_direction"`
}
