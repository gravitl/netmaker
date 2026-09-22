package models

import (
	"net"

	"github.com/gravitl/netmaker/schema"
)

type AllowedTrafficDirection = schema.AllowedTrafficDirection
type Protocol = schema.Protocol
type AclPolicyType = schema.AclPolicyType
type AclPolicyTag = schema.AclPolicyTag
type AclGroupType = schema.AclGroupType
type Acl = schema.Acl

const (
	TrafficDirectionUni = schema.TrafficDirectionUni
	TrafficDirectionBi  = schema.TrafficDirectionBi

	ALL  = schema.ALL
	UDP  = schema.UDP
	TCP  = schema.TCP
	ICMP = schema.ICMP

	UserPolicy   = schema.UserPolicy
	DevicePolicy = schema.DevicePolicy

	UserAclID                = schema.UserAclID
	UserGroupAclID           = schema.UserGroupAclID
	NodeTagID                = schema.NodeTagID
	NodeID                   = schema.NodeID
	EgressRange              = schema.EgressRange
	EgressID                 = schema.EgressID
	NetmakerIPAclID          = schema.NetmakerIPAclID
	NetmakerSubNetRangeAClID = schema.NetmakerSubNetRangeAClID

	Http        = "HTTP"
	Https       = "HTTPS"
	AllTCP      = "All TCP"
	AllUDP      = "All UDP"
	ICMPService = "ICMP"
	SSH         = "SSH"
	Custom      = "Custom"
	Any         = "Any"
)

type UpdateAclRequest struct {
	Acl
	NewName string `json:"new_name"`
}

type AclPolicy struct {
	TypeID        AclPolicyType
	PrefixTagUser AclGroupType
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
	ID              string                  `json:"id"`
	IPList          []net.IPNet             `json:"ip_list"`
	IP6List         []net.IPNet             `json:"ip6_list"`
	AllowedProtocol Protocol                `json:"allowed_protocols"`
	AllowedPorts    []string                `json:"allowed_ports"`
	Direction       AllowedTrafficDirection `json:"direction"`
	Dst             []net.IPNet             `json:"dst"`
	Dst6            []net.IPNet             `json:"dst6"`
	Allowed         bool
}
