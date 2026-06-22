package models

import (
	"net"

	"github.com/gravitl/netmaker/schema"
)

// Non-KV string constants that stay in models.
const (
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
	schema.Acl
	NewName string `json:"new_name"`
}

type AclPolicyTypes struct {
	ProtocolTypes []ProtocolType
	RuleTypes     []schema.AclPolicyType `json:"policy_types"`
	SrcGroupTypes []schema.AclGroupType  `json:"src_grp_types"`
	DstGroupTypes []schema.AclGroupType  `json:"dst_grp_types"`
}

type ProtocolType struct {
	Name             string            `json:"name"`
	AllowedProtocols []schema.Protocol `json:"allowed_protocols"`
	PortRange        string            `json:"port_range"`
	AllowPortSetting bool              `json:"allow_port_setting"`
}

type AclRule struct {
	ID              string                         `json:"id"`
	IPList          []net.IPNet                    `json:"ip_list"`
	IP6List         []net.IPNet                    `json:"ip6_list"`
	AllowedProtocol schema.Protocol                `json:"allowed_protocols"` // tcp, udp, etc.
	AllowedPorts    []string                       `json:"allowed_ports"`
	Direction       schema.AllowedTrafficDirection `json:"direction"` // single or two-way
	Dst             []net.IPNet                    `json:"dst"`
	Dst6            []net.IPNet                    `json:"dst6"`
	Allowed         bool
}
