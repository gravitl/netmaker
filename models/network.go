package models

import (
	"strings"
	"time"

	"github.com/gravitl/netmaker/servercfg"
)

// Network Struct - contains info for a given unique network
//At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	AddressRange        string      `json:"addressrange" bson:"addressrange" validate:"required,cidr"`
	AddressRange6       string      `json:"addressrange6" bson:"addressrange6" validate:"regexp=^s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*(\/([0-9]|[1-9][0-9]|1[0-1][0-9]|12[0-8]))?$"`
	DisplayName         string      `json:"displayname,omitempty" bson:"displayname,omitempty" validate:"omitempty,min=1,max=20,displayname_valid"`
	NetID               string      `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
	NodesLastModified   int64       `json:"nodeslastmodified" bson:"nodeslastmodified"`
	NetworkLastModified int64       `json:"networklastmodified" bson:"networklastmodified"`
	DefaultInterface    string      `json:"defaultinterface" bson:"defaultinterface" validate:"min=1,max=15"`
	DefaultListenPort   int32       `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,min=1024,max=65535"`
	NodeLimit           int32       `json:"nodelimit" bson:"nodelimit"`
	DefaultPostUp       string      `json:"defaultpostup" bson:"defaultpostup"`
	DefaultPostDown     string      `json:"defaultpostdown" bson:"defaultpostdown"`
	KeyUpdateTimeStamp  int64       `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
	DefaultKeepalive    int32       `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	DefaultSaveConfig   string      `json:"defaultsaveconfig" bson:"defaultsaveconfig" validate:"checkyesorno"`
	AccessKeys          []AccessKey `json:"accesskeys" bson:"accesskeys"`
	AllowManualSignUp   string      `json:"allowmanualsignup" bson:"allowmanualsignup" validate:"checkyesorno"`
	IsLocal             string      `json:"islocal" bson:"islocal" validate:"checkyesorno"`
	IsDualStack         string      `json:"isdualstack" bson:"isdualstack" validate:"checkyesorno"`
	IsIPv4              string      `json:"isipv4" bson:"isipv4" validate:"checkyesorno"`
	IsIPv6              string      `json:"isipv6" bson:"isipv6" validate:"checkyesorno"`
	IsGRPCHub           string      `json:"isgrpchub" bson:"isgrpchub" validate:"checkyesorno"`
	LocalRange          string      `json:"localrange" bson:"localrange" validate:"omitempty,cidr"`

	// checkin interval is depreciated at the network level. Set on server with CHECKIN_INTERVAL
	DefaultCheckInInterval int32        `json:"checkininterval,omitempty" bson:"checkininterval,omitempty" validate:"omitempty,numeric,min=2,max=100000"`
	DefaultUDPHolePunch    string       `json:"defaultudpholepunch" bson:"defaultudpholepunch" validate:"checkyesorno"`
	DefaultExtClientDNS    string       `json:"defaultextclientdns" bson:"defaultextclientdns"`
	DefaultMTU             int32        `json:"defaultmtu" bson:"defaultmtu"`
	DefaultServerAddrs     []ServerAddr `json:"defaultserveraddrs" bson:"defaultserveraddrs" yaml:"defaultserveraddrs"`
}

// SaveData - sensitive fields of a network that should be kept the same
type SaveData struct { // put sensitive fields here
	NetID string `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
}

// Network.DisplayNameInNetworkCharSet - checks if displayname uses valid characters
func (network *Network) DisplayNameInNetworkCharSet() bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_./;% ^#()!@$*"

	for _, char := range network.DisplayName {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
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
		if servercfg.IsClientMode() != "off" {
			network.DefaultUDPHolePunch = "yes"
		} else {
			network.DefaultUDPHolePunch = "no"
		}
	}
	if network.IsLocal == "" {
		network.IsLocal = "no"
	}
	if network.IsGRPCHub == "" {
		network.IsGRPCHub = "no"
	}
	if network.DisplayName == "" {
		network.DisplayName = network.NetID
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
	if network.DefaultSaveConfig == "" {
		network.DefaultSaveConfig = "no"
	}
	if network.DefaultKeepalive == 0 {
		network.DefaultKeepalive = 20
	}
	//Check-In Interval for Nodes, In Seconds
	if network.DefaultCheckInInterval == 0 {
		network.DefaultCheckInInterval = 30
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
}
