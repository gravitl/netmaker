package models

import (
	//  "../mongoconn"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

//Network Struct
//At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	ID           primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	AddressRange string             `json:"addressrange" bson:"addressrange" validate:"required,cidr"`
	// bug in validator --- required_with does not work with bools  issue#683
	//	AddressRange6          string             `json:"addressrange6" bson:"addressrange6" validate:"required_with=isdualstack true,cidrv6"`
	AddressRange6 string `json:"addressrange6" bson:"addressrange6" validate:"addressrange6_valid"`
	//can't have min=1 with omitempty
	DisplayName         string      `json:"displayname,omitempty" bson:"displayname,omitempty" validate:"omitempty,alphanum,min=2,max=20,displayname_unique"`
	NetID               string      `json:"netid" bson:"netid" validate:"required,alphanum,min=1,max=12,netid_valid"`
	NodesLastModified   int64       `json:"nodeslastmodified" bson:"nodeslastmodified"`
	NetworkLastModified int64       `json:"networklastmodified" bson:"networklastmodified"`
	DefaultInterface    string      `json:"defaultinterface" bson:"defaultinterface"`
	DefaultListenPort   int32       `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,min=1024,max=65535"`
	DefaultPostUp       string      `json:"defaultpostup" bson:"defaultpostup"`
	DefaultPostDown     string      `json:"defaultpostdown" bson:"defaultpostdown"`
	KeyUpdateTimeStamp  int64       `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
	DefaultKeepalive    int32       `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	DefaultSaveConfig   *bool       `json:"defaultsaveconfig" bson:"defaultsaveconfig"`
	AccessKeys          []AccessKey `json:"accesskeys" bson:"accesskeys"`
	AllowManualSignUp   *bool       `json:"allowmanualsignup" bson:"allowmanualsignup"`
	IsLocal             *bool       `json:"islocal" bson:"islocal"`
	IsDualStack         *bool       `json:"isdualstack" bson:"isdualstack"`
	LocalRange          string      `json:"localrange" bson:"localrange" validate:"omitempty,cidr"`
	//can't have min=1 with omitempty
	DefaultCheckInInterval int32 `json:"checkininterval,omitempty" bson:"checkininterval,omitempty" validate:"omitempty,numeric,min=2,max=100000"`
}

//TODO:
//Not  sure if we  need the below two functions. Got rid  of one of the calls. May want  to revisit
func (network *Network) SetNodesLastModified() {
	network.NodesLastModified = time.Now().Unix()
}

func (network *Network) SetNetworkLastModified() {
	network.NetworkLastModified = time.Now().Unix()
}

func (network *Network) SetDefaults() {
	if network.DisplayName == "" {
		network.DisplayName = network.NetID
	}
	if network.DefaultInterface == "" {
		network.DefaultInterface = "nm-" + network.NetID
	}
	if network.DefaultListenPort == 0 {
		network.DefaultListenPort = 51821
	}
	if network.DefaultPostDown == "" {

	}
	if network.DefaultSaveConfig == nil {
		defaultsave := true
		network.DefaultSaveConfig = &defaultsave
	}
	if network.DefaultKeepalive == 0 {
		network.DefaultKeepalive = 20
	}
	if network.DefaultPostUp == "" {
	}
	//Check-In Interval for Nodes, In Seconds
	if network.DefaultCheckInInterval == 0 {
		network.DefaultCheckInInterval = 30
	}
	if network.AllowManualSignUp == nil {
		signup := false
		network.AllowManualSignUp = &signup
	}
}
