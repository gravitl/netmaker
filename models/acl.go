package models

import (
	"net"
	"time"
)

// AllowedTrafficDirection - allowed direction of traffic
type AllowedTrafficDirection int

const (
	// TrafficDirectionUni implies traffic is only allowed in one direction (src --> dst)
	TrafficDirectionUni AllowedTrafficDirection = iota
	// TrafficDirectionBi implies traffic is allowed both direction (src <--> dst )
	TrafficDirectionBi
)

// Protocol - allowed protocol
type Protocol int

const (
	ALL Protocol = iota
	UDP
	TCP
	ICMP
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
	ID               string                  `json:"id"`
	Default          bool                    `json:"default"`
	MetaData         string                  `json:"meta_data"`
	Name             string                  `json:"name"`
	NetworkID        NetworkID               `json:"network_id"`
	RuleType         AclPolicyType           `json:"policy_type"`
	Src              []AclPolicyTag          `json:"src_type"`
	Dst              []AclPolicyTag          `json:"dst_type"`
	Proto            []Protocol              `json:"protocol"` // tcp, udp, etc.
	Port             []int                   `json:"ports"`
	AllowedDirection AllowedTrafficDirection `json:"allowed_traffic_direction"`
	Enabled          bool                    `json:"enabled"`
	CreatedBy        string                  `json:"created_by"`
	CreatedAt        time.Time               `json:"created_at"`
}

type AclPolicyTypes struct {
	ProtocolTypes []ProtocolType
	RuleTypes     []AclPolicyType `json:"policy_types"`
	SrcGroupTypes []AclGroupType  `json:"src_grp_types"`
	DstGroupTypes []AclGroupType  `json:"dst_grp_types"`
}

type ProtocolType struct {
	Name             string     `json:"name"`
	AllowedProtocols []Protocol `json:"allowed_protocols"`
	PortRange        string     `json:"port_range"`
	AllowPortSetting bool       `json:"allow_port_setting"`
}

type AclRule struct {
	SrcIP     net.IPNet
	SrcIP6    net.IPNet
	Proto     []Protocol // tcp, udp, etc.
	Port      []int
	Direction AllowedTrafficDirection // inbound or outbound
	Allowed   bool
}
