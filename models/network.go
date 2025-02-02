package models

import (
	"net"
	"time"
)

// Network Struct - contains info for a given unique network
// At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	AddressRange        string `json:"addressrange" bson:"addressrange" validate:"omitempty,cidrv4"`
	AddressRange6       string `json:"addressrange6" bson:"addressrange6" validate:"omitempty,cidrv6"`
	NetID               string `json:"netid" bson:"netid" validate:"required,min=1,max=32,netid_valid"`
	NodesLastModified   int64  `json:"nodeslastmodified" bson:"nodeslastmodified" swaggertype:"primitive,integer" format:"int64"`
	NetworkLastModified int64  `json:"networklastmodified" bson:"networklastmodified" swaggertype:"primitive,integer" format:"int64"`
	DefaultInterface    string `json:"defaultinterface" bson:"defaultinterface" validate:"min=1,max=35"`
	DefaultListenPort   int32  `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,min=1024,max=65535"`
	NodeLimit           int32  `json:"nodelimit" bson:"nodelimit"`
	DefaultPostDown     string `json:"defaultpostdown" bson:"defaultpostdown"`
	DefaultKeepalive    int32  `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	AllowManualSignUp   string `json:"allowmanualsignup" bson:"allowmanualsignup" validate:"checkyesorno"`
	IsIPv4              string `json:"isipv4" bson:"isipv4" validate:"checkyesorno"`
	IsIPv6              string `json:"isipv6" bson:"isipv6" validate:"checkyesorno"`
	DefaultUDPHolePunch string `json:"defaultudpholepunch" bson:"defaultudpholepunch" validate:"checkyesorno"`
	DefaultMTU          int32  `json:"defaultmtu" bson:"defaultmtu"`
	DefaultACL          string `json:"defaultacl" bson:"defaultacl" yaml:"defaultacl" validate:"checkyesorno"`
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
	return
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
