package models

import (
	"net"
	"time"
)

// Network Struct - contains info for a given unique network
// At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	AddressRange        string   `json:"addressrange" bson:"addressrange" validate:"omitempty,cidrv4"`
	AddressRange6       string   `json:"addressrange6" bson:"addressrange6" validate:"omitempty,cidrv6"`
	NetID               string   `json:"netid" bson:"netid" validate:"required,min=1,max=32,netid_valid"`
	NodesLastModified   int64    `json:"nodeslastmodified" bson:"nodeslastmodified" swaggertype:"primitive,integer" format:"int64"`
	NetworkLastModified int64    `json:"networklastmodified" bson:"networklastmodified" swaggertype:"primitive,integer" format:"int64"`
	DefaultInterface    string   `json:"defaultinterface" bson:"defaultinterface" validate:"min=1,max=35"`
	DefaultListenPort   int32    `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,min=1024,max=65535"`
	NodeLimit           int32    `json:"nodelimit" bson:"nodelimit"`
	DefaultPostDown     string   `json:"defaultpostdown" bson:"defaultpostdown"`
	DefaultKeepalive    int32    `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	AllowManualSignUp   string   `json:"allowmanualsignup" bson:"allowmanualsignup" validate:"checkyesorno"`
	IsIPv4              string   `json:"isipv4" bson:"isipv4" validate:"checkyesorno"`
	IsIPv6              string   `json:"isipv6" bson:"isipv6" validate:"checkyesorno"`
	DefaultUDPHolePunch string   `json:"defaultudpholepunch" bson:"defaultudpholepunch" validate:"checkyesorno"`
	DefaultMTU          int32    `json:"defaultmtu" bson:"defaultmtu"`
	DefaultACL          string   `json:"defaultacl" bson:"defaultacl" yaml:"defaultacl" validate:"checkyesorno"`
	NameServers         []string `json:"dns_nameservers"`
	AutoJoin            string   `json:"auto_join"`
	AutoRemove          string   `json:"auto_remove"`
	AutoRemoveTags      []string `json:"auto_remove_tags"`
	AutoRemoveThreshold int      `json:"auto_remove_threshold_mins"`
	JITEnabled          string   `json:"jit_enabled" bson:"jit_enabled" validate:"checkyesorno"`
	// VirtualNATPoolIPv4 is the IPv4 CIDR pool from which virtual NAT ranges are allocated for egress gateways
	VirtualNATPoolIPv4 string `json:"virtual_nat_pool_ipv4"`
	// VirtualNATSitePrefixLenIPv4 is the prefix length (e.g., 24) for individual site allocations from the IPv4 virtual NAT pool
	VirtualNATSitePrefixLenIPv4 int `json:"virtual_nat_site_prefixlen_ipv4"`
}

// SaveData - sensitive fields of a network that should be kept the same
type SaveData struct { // put sensitive fields here
	NetID string `json:"netid" bson:"netid" validate:"required,min=1,max=32,netid_valid"`
}

// Network.SetNodesLastModified - sets nodes last modified on network, depricated
func (network *Network) SetNodesLastModified() {
	network.NodesLastModified = time.Now().Unix()
}

// Network.SetNetworkLastModified - sets network last modified time
func (network *Network) SetNetworkLastModified() {
	network.NetworkLastModified = time.Now().Unix()
}

// Network.SetDefaults - sets default values for a network struct
func (network *Network) SetDefaults() (upsert bool) {
	if network.DefaultUDPHolePunch == "" {
		network.DefaultUDPHolePunch = "no"
		upsert = true
	}
	if network.DefaultInterface == "" {
		if len(network.NetID) < 33 {
			network.DefaultInterface = "nm-" + network.NetID
		} else {
			network.DefaultInterface = network.NetID
		}
		upsert = true
	}
	if network.DefaultListenPort == 0 {
		network.DefaultListenPort = 51821
		upsert = true
	}
	if network.NodeLimit == 0 {
		network.NodeLimit = 999999999
		upsert = true
	}
	if network.DefaultKeepalive == 0 {
		network.DefaultKeepalive = 20
		upsert = true
	}
	if network.AllowManualSignUp == "" {
		network.AllowManualSignUp = "no"
		upsert = true
	}

	if network.IsIPv4 == "" {
		network.IsIPv4 = "yes"
		upsert = true
	}

	if network.IsIPv6 == "" {
		network.IsIPv6 = "no"
		upsert = true
	}

	if network.DefaultMTU == 0 {
		network.DefaultMTU = 1280
		upsert = true
	}

	if network.DefaultACL == "" {
		network.DefaultACL = "yes"
		upsert = true
	}

	if network.JITEnabled == "" {
		network.JITEnabled = "no"
		upsert = true
	}
	return
}

// AssignVirtualNATDefaults determines safe defaults based on VPN CIDR
func (network *Network) AssignVirtualNATDefaults(vpnCIDR string, networkID string) {
	const (
		cgnatCIDR        = "100.64.0.0/10"
		fallbackIPv4Pool = "198.18.0.0/15"

		defaultIPv4SitePrefix = 24
	)

	// Parse CGNAT CIDR (should always succeed, but check for safety)
	_, cgnatNet, err := net.ParseCIDR(cgnatCIDR)
	if err != nil {
		// Fallback to default pool if CGNAT parsing fails (shouldn't happen)
		network.VirtualNATPoolIPv4 = fallbackIPv4Pool
		network.VirtualNATSitePrefixLenIPv4 = defaultIPv4SitePrefix
		return
	}

	var virtualIPv4Pool string
	// Parse VPN CIDR - if it fails or is empty, use fallback
	if vpnCIDR == "" {
		virtualIPv4Pool = fallbackIPv4Pool
	} else {
		_, vpnNet, err := net.ParseCIDR(vpnCIDR)
		if err != nil || vpnNet == nil {
			// Invalid VPN CIDR, use fallback
			virtualIPv4Pool = fallbackIPv4Pool
		} else if !cidrOverlaps(vpnNet, cgnatNet) {
			// Safe to reuse VPN CIDR for Virtual NAT
			virtualIPv4Pool = vpnCIDR
		} else {
			// VPN is CGNAT — must not reuse
			virtualIPv4Pool = fallbackIPv4Pool
		}
	}

	network.VirtualNATPoolIPv4 = virtualIPv4Pool
	network.VirtualNATSitePrefixLenIPv4 = defaultIPv4SitePrefix

}
func cidrOverlaps(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Contains(b.IP) || b.Contains(a.IP)
}

func (network *Network) GetNetworkNetworkCIDR4() *net.IPNet {
	if network.AddressRange == "" {
		return nil
	}
	_, netCidr, _ := net.ParseCIDR(network.AddressRange)
	return netCidr
}
func (network *Network) GetNetworkNetworkCIDR6() *net.IPNet {
	if network.AddressRange6 == "" {
		return nil
	}
	_, netCidr, _ := net.ParseCIDR(network.AddressRange6)
	return netCidr
}

type NetworkStatResp struct {
	Network
	Hosts int `json:"hosts"`
}
