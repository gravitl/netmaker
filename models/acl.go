package models

import (
	"fmt"
	"time"
)

type AclID string

func (aID AclID) String() string {
	return string(aID)
}

func (a *Acl) GetID(netID NetworkID, name string) {
	a.ID = AclID(fmt.Sprintf("%s.%s", netID.String(), name))
}

func FormatAclID(netID NetworkID, name string) AclID {
	return AclID(fmt.Sprintf("%s.%s", netID.String(), name))
}

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

type AclPolicyTag struct {
	ID    AclGroupType `json:"id"`
	Value string       `json:"value"`
}

type AclGroupType string

const (
	UserAclID                AclGroupType = "user"
	UserGroupAclID           AclGroupType = "user-group"
	DeviceAclID              AclGroupType = "tag"
	NetmakerIPAclID          AclGroupType = "ip"
	NetmakerSubNetRangeAClID AclGroupType = "ipset"
)

func (g AclGroupType) String() string {
	return string(g)
}

type UpdateAclRequest struct {
	Acl
	NewName string `json:"new_name"`
}

type AclPolicy struct {
	TypeID        AclPolicyType
	PrefixTagUser AclGroupType
}

type Acl struct {
	ID               AclID                   `json:"id"`
	Default          bool                    `json:"default"`
	Name             string                  `json:"name"`
	NetworkID        NetworkID               `json:"network_id"`
	RuleType         AclPolicyType           `json:"policy_type"`
	Src              []AclPolicyTag          `json:"src_type"`
	Dst              []AclPolicyTag          `json:"dst_type"`
	AllowedDirection AllowedTrafficDirection `json:"allowed_traffic_direction"`
	Enabled          bool                    `json:"enabled"`
	CreatedBy        string                  `json:"created_by"`
	CreatedAt        time.Time               `json:"created_at"`
}
