package models

import (
	"crypto/ed25519"
	"time"
)

// Network Struct - contains info for a given unique network
//At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	AddressRange        string              `json:"addressrange" bson:"addressrange" validate:"required,cidr"`
	AddressRange6       string              `json:"addressrange6" bson:"addressrange6" validate:"regexp=^s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*(\/([0-9]|[1-9][0-9]|1[0-1][0-9]|12[0-8]))?$"`
	NetID               string              `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
	NodesLastModified   int64               `json:"nodeslastmodified" bson:"nodeslastmodified"`
	NetworkLastModified int64               `json:"networklastmodified" bson:"networklastmodified"`
	DefaultInterface    string              `json:"defaultinterface" bson:"defaultinterface" validate:"min=1,max=15"`
	DefaultListenPort   uint16              `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,min=1024,max=65535"`
	NodeLimit           int32               `json:"nodelimit" bson:"nodelimit"`
	DefaultPostUp       string              `json:"defaultpostup" bson:"defaultpostup"`
	DefaultPostDown     string              `json:"defaultpostdown" bson:"defaultpostdown"`
	DefaultKeepalive    int32               `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	AccessKeys          []AccessKey         `json:"accesskeys" bson:"accesskeys"`
	AllowedKeys         []ed25519.PublicKey `json:"allowedkeys" bson:"allowedkeys"`
	AllowManualSignUp   string              `json:"allowmanualsignup" bson:"allowmanualsignup" validate:"checkyesorno"`
	IsLocal             string              `json:"islocal" bson:"islocal" validate:"checkyesorno"`
	IsDualStack         string              `json:"isdualstack" bson:"isdualstack" validate:"checkyesorno"`
	IsIPv4              string              `json:"isipv4" bson:"isipv4" validate:"checkyesorno"`
	IsIPv6              string              `json:"isipv6" bson:"isipv6" validate:"checkyesorno"`
	IsPointToSite       string              `json:"ispointtosite" bson:"ispointtosite" validate:"checkyesorno"`
	IsComms             string              `json:"iscomms" bson:"iscomms" validate:"checkyesorno"`
	LocalRange          string              `json:"localrange" bson:"localrange" validate:"omitempty,cidr"`
	DefaultUDPHolePunch string              `json:"defaultudpholepunch" bson:"defaultudpholepunch" validate:"checkyesorno"`
	DefaultExtClientDNS string              `json:"defaultextclientdns" bson:"defaultextclientdns"`
	DefaultMTU          int32               `json:"defaultmtu" bson:"defaultmtu"`
	// consider removing - may be depreciated
	DefaultServerAddrs []ServerAddr `json:"defaultserveraddrs" bson:"defaultserveraddrs" yaml:"defaultserveraddrs"`
	DefaultACL         string       `json:"defaultacl" bson:"defaultacl" yaml:"defaultacl" validate:"checkyesorno"`
}

// SaveData - sensitive fields of a network that should be kept the same
type SaveData struct { // put sensitive fields here
	NetID string `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
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
func (network *Network) SetDefaults() {
	if network.DefaultUDPHolePunch == "" {
		network.DefaultUDPHolePunch = "no"
	}
	if network.IsLocal == "" {
		network.IsLocal = "no"
	}
	if network.IsPointToSite == "" {
		network.IsPointToSite = "no"
	}
	if network.IsComms == "" {
		network.IsComms = "no"
	}
	if network.DefaultInterface == "" {
		if len(network.NetID) < 13 {
			network.DefaultInterface = "nm-" + network.NetID
		} else {
			network.DefaultInterface = network.NetID
		}
	}
	if network.DefaultListenPort == 0 {
		network.DefaultListenPort = 51821
	}
	if network.NodeLimit == 0 {
		network.NodeLimit = 999999999
	}
	if network.DefaultKeepalive == 0 {
		network.DefaultKeepalive = 20
	}
	if network.AllowManualSignUp == "" {
		network.AllowManualSignUp = "no"
	}
	if network.IsDualStack == "" {
		network.IsDualStack = "no"
	}
	if network.IsDualStack == "yes" {
		network.IsIPv6 = "yes"
		network.IsIPv4 = "yes"
	} else {
		network.IsIPv6 = "no"
		network.IsIPv4 = "yes"
	}

	if network.DefaultMTU == 0 {
		network.DefaultMTU = 1280
	}

	if network.DefaultACL == "" {
		network.DefaultACL = "yes"
	}
}
