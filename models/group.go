package models

import (
//  "../mongoconn"
  "go.mongodb.org/mongo-driver/bson/primitive"
  "time"
)

//Group Struct
//At  some point, need to replace all instances of Name with something else like  Identifier
type Group struct {
	ID	primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	AddressRange	string `json:"addressrange" bson:"addressrange" validate:"required,addressrange_valid"`
	DisplayName string `json:"displayname,omitempty" bson:"displayname,omitempty" validate:"omitempty,displayname_unique,min=1,max=100"`
	NameID string `json:"nameid" bson:"nameid" validate:"required,nameid_valid,min=1,max=12"`
	NodesLastModified	int64 `json:"nodeslastmodified" bson:"nodeslastmodified"`
	GroupLastModified int64 `json:"grouplastmodified" bson:"grouplastmodified"`
	DefaultInterface	string `json:"defaulinterface" bson:"defaultinterface"`
        DefaultListenPort      int32 `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,numeric,min=1024,max=65535"`
        DefaultPostUp  string `json:"defaultpostup" bson:"defaultpostup"`
        DefaultPreUp   string `json:"defaultpreup" bson:"defaultpreup"`
        KeyUpdateTimeStamp      int64 `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
        DefaultKeepalive int32 `json:"defaultkeepalive" bson:"defaultkeepalive" validate: "omitempty,numeric,max=1000"`
        DefaultSaveConfig      *bool `json:"defaultsaveconfig" bson:"defaultsaveconfig"`
	AccessKeys	[]AccessKey `json:"accesskeys" bson:"accesskeys"`
	AllowManualSignUp *bool `json:"allowmanualsignup" bson:"allowmanualsignup"`
	DefaultCheckInInterval int32 `json:"checkininterval,omitempty" bson:"checkininterval,omitempty" validate:"omitempty,numeric,min=1,max=100000"`
}

//TODO:
//Not  sure if we  need the below two functions. Got rid  of one of the calls. May want  to revisit
func(group *Group) SetNodesLastModified(){
        group.NodesLastModified = time.Now().Unix()
}

func(group *Group) SetGroupLastModified(){
        group.GroupLastModified = time.Now().Unix()
}

func(group *Group) SetDefaults(){
    if group.DisplayName == "" {
        group.DisplayName = group.NameID
    }
    if group.DefaultInterface == "" {
	group.DefaultInterface = "nm-" + group.NameID
    }
    if group.DefaultListenPort == 0 {
        group.DefaultListenPort = 51821
    }
    if group.DefaultPreUp == "" {

    }
    if group.DefaultSaveConfig == nil {
	defaultsave := true
        group.DefaultSaveConfig = &defaultsave
    }
    if group.DefaultKeepalive == 0 {
        group.DefaultKeepalive = 20
    }
    if group.DefaultPostUp == "" {
    }
    //Check-In Interval for Nodes, In Seconds
    if group.DefaultCheckInInterval == 0 {
        group.DefaultCheckInInterval = 30
    }
    if group.AllowManualSignUp == nil {
	signup := false
        group.AllowManualSignUp = &signup
    }
}
