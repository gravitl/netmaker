package models

import (
	"net"

	"github.com/gravitl/netmaker/schema"
)

type AllowedTrafficDirection = schema.AllowedTrafficDirection
type Protocol = schema.Protocol
type AclPolicyType = schema.AclPolicyType
type AclAccessType = schema.AclAccessType
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

	NetworkAccess = schema.NetworkAccess
	ManagedAccess = schema.ManagedAccess

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
	ManagedSSH  = "Managed SSH"
	Any         = "Any"

	ManagedSSHPort = "22022"

	SSHUserNamespace = "nm:"
	SSHUserAny       = "nm:any"    // any OS user
	SSHUserRoot      = "nm:root"   // only uid 0 accounts
	SSHUserNoRoot    = "nm:noroot" // any account except uid 0
)

// SSHUserSpecials are every reserved ssh_users value.
var SSHUserSpecials = []string{SSHUserAny, SSHUserRoot, SSHUserNoRoot}

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
	AccessTypes   []AclAccessType `json:"access_types"`
	SrcGroupTypes []AclGroupType  `json:"src_grp_types"`
	DstGroupTypes []AclGroupType  `json:"dst_grp_types"`
}

type ProtocolType struct {
	Name             string        `json:"name"`
	AccessType       AclAccessType `json:"access_type"`
	AllowedProtocols []Protocol    `json:"allowed_protocols"`
	PortRange        string        `json:"port_range"`
	AllowPortSetting bool          `json:"allow_port_setting"`
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
