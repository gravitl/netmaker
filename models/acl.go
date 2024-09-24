package models

import (
	"github.com/google/uuid"
)

// AllowedTrafficDirection - allowed direction of traffic
type AllowedTrafficDirection int

const (
	// TrafficDirectionUni implies traffic is only allowed in one direction (src --> dst)
	TrafficDirectionUni AllowedTrafficDirection = iota
	// TrafficDirectionBi implies traffic is allowed both direction (src <--> dst )
	TrafficDirectionBi
)

type AclPolicyType string

const (
	UserPolicy   AclPolicyType = "user-policy"
	DevicePolicy AclPolicyType = "device-policy"
)

type Acl struct {
	ID               uuid.UUID               `json:"id"`
	Name             string                  `json:"name"`
	NetworkID        NetworkID               `json:"network_id"`
	RuleType         AclPolicyType           `json:"policy_type"`
	Src              []string                `json:"src_type"`
	Dst              []string                `json:"dst_type"`
	AllowedDirection AllowedTrafficDirection `json:"allowed_traffic_direction"`
	Enabled          bool                    `json:"enabled"`
}
