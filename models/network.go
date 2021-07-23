package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
)

//Network Struct
//At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	AddressRange           string      `json:"addressrange" bson:"addressrange" validate:"required,cidr"`
	AddressRange6          string      `json:"addressrange6" bson:"addressrange6" validate:"regexp=^s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*(\/([0-9]|[1-9][0-9]|1[0-1][0-9]|12[0-8]))?$"`
	DisplayName            string      `json:"displayname,omitempty" bson:"displayname,omitempty" validate:"omitempty,min=1,max=20,displayname_valid"`
	NetID                  string      `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
	NodesLastModified      int64       `json:"nodeslastmodified" bson:"nodeslastmodified"`
	NetworkLastModified    int64       `json:"networklastmodified" bson:"networklastmodified"`
	DefaultInterface       string      `json:"defaultinterface" bson:"defaultinterface" validate:"min=1,max=15"`
	DefaultListenPort      int32       `json:"defaultlistenport,omitempty" bson:"defaultlistenport,omitempty" validate:"omitempty,min=1024,max=65535"`
	NodeLimit              int32       `json:"nodelimit" bson:"nodelimit"`
	DefaultPostUp          string      `json:"defaultpostup" bson:"defaultpostup"`
	DefaultPostDown        string      `json:"defaultpostdown" bson:"defaultpostdown"`
	KeyUpdateTimeStamp     int64       `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
	DefaultKeepalive       int32       `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	DefaultSaveConfig      string      `json:"defaultsaveconfig" bson:"defaultsaveconfig" validate:"regexp=^(yes|no)$"`
	AccessKeys             []AccessKey `json:"accesskeys" bson:"accesskeys"`
	AllowManualSignUp      string      `json:"allowmanualsignup" bson:"allowmanualsignup" validate:"regexp=^(yes|no)$"`
	IsLocal                string      `json:"islocal" bson:"islocal" validate:"regexp=^(yes|no)$"`
	IsDualStack            string      `json:"isdualstack" bson:"isdualstack" validate:"regexp=^(yes|no)$"`
	IsIPv4                 string      `json:"isipv4" bson:"isipv4" validate:"regexp=^(yes|no)$"`
	IsIPv6                 string      `json:"isipv6" bson:"isipv6" validate:"regexp=^(yes|no)$"`
	IsGRPCHub              string      `json:"isgrpchub" bson:"isgrpchub" validate:"regexp=^(yes|no)$"`
	LocalRange             string      `json:"localrange" bson:"localrange" validate:"omitempty,cidr"`
	DefaultCheckInInterval int32       `json:"checkininterval,omitempty" bson:"checkininterval,omitempty" validate:"omitempty,numeric,min=2,max=100000"`
	DefaultUDPHolePunch    string      `json:"defaultudpholepunch" bson:"defaultudpholepunch" validate:"regexp=^(yes|no)$"`
}

type SaveData struct { // put sensitive fields here
	NetID string `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
}

var FIELDS = map[string][]string{
	// "id":                  {"ID", "string"},
	"addressrange":        {"AddressRange", "string"},
	"addressrange6":       {"AddressRange6", "string"},
	"displayname":         {"DisplayName", "string"},
	"netid":               {"NetID", "string"},
	"nodeslastmodified":   {"NodesLastModified", "int64"},
	"networklastmodified": {"NetworkLastModified", "int64"},
	"defaultinterface":    {"DefaultInterface", "string"},
	"defaultlistenport":   {"DefaultListenPort", "int32"},
	"nodelimit":           {"NodeLimit", "int32"},
	"defaultpostup":       {"DefaultPostUp", "string"},
	"defaultpostdown":     {"DefaultPostDown", "string"},
	"keyupdatetimestamp":  {"KeyUpdateTimeStamp", "int64"},
	"defaultkeepalive":    {"DefaultKeepalive", "int32"},
	"defaultsaveconfig":   {"DefaultSaveConfig", "string"},
	"accesskeys":          {"AccessKeys", "[]AccessKey"},
	"allowmanualsignup":   {"AllowManualSignUp", "string"},
	"islocal":             {"IsLocal", "string"},
	"isdualstack":         {"IsDualStack", "string"},
	"isipv4":              {"IsIPv4", "string"},
	"isipv6":              {"IsIPv6", "string"},
	"isgrpchub":           {"IsGRPCHub", "string"},
	"localrange":          {"LocalRange", "string"},
	"checkininterval":     {"DefaultCheckInInterval", "int32"},
	"defaultudpholepunch": {"DefaultUDPHolePunch", "string"},
}

func (network *Network) FieldExists(field string) bool {
	return len(FIELDS[field]) > 0
}

func (network *Network) NetIDInNetworkCharSet() bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_."

	for _, char := range network.NetID {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

func (network *Network) DisplayNameInNetworkCharSet() bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_./;% ^#()!@$*"

	for _, char := range network.DisplayName {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// Anyway, returns all the networks
func GetNetworks() ([]Network, error) {
	var networks []Network

	collection, err := database.FetchRecords(database.NETWORKS_TABLE_NAME)

	if err != nil {
		return networks, err
	}

	for _, value := range collection {
		var network Network
		if err := json.Unmarshal([]byte(value), &network); err != nil {
			return networks, err
		}
		// add network our array
		networks = append(networks, network)
	}

	return networks, err
}

func (network *Network) IsNetworkDisplayNameUnique() (bool, error) {

	isunique := true

	records, err := GetNetworks()

	if err != nil {
		return false, err
	}

	for i := 0; i < len(records); i++ {

		if network.NetID == records[i].DisplayName {
			isunique = false
		}
	}

	return isunique, nil
}

//Checks to see if any other networks have the same name (id)
func (network *Network) IsNetworkNameUnique() (bool, error) {

	isunique := true

	dbs, err := GetNetworks()

	if err != nil {
		return false, err
	}

	for i := 0; i < len(dbs); i++ {

		if network.NetID == dbs[i].NetID {
			isunique = false
		}
	}

	return isunique, nil
}

func (network *Network) Validate() error {
	v := validator.New()
	_ = v.RegisterValidation("netid_valid", func(fl validator.FieldLevel) bool {
		isFieldUnique, _ := network.IsNetworkNameUnique()
		inCharSet := network.NetIDInNetworkCharSet()
		return isFieldUnique && inCharSet
	})
	//
	_ = v.RegisterValidation("displayname_valid", func(fl validator.FieldLevel) bool {
		isFieldUnique, _ := network.IsNetworkDisplayNameUnique()
		inCharSet := network.DisplayNameInNetworkCharSet()
		return isFieldUnique && inCharSet
	})

	err := v.Struct(network)
	return err
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
	if network.DefaultUDPHolePunch == "" {
		network.DefaultUDPHolePunch = "yes"
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
}

func (currentNetwork *Network) Update(newNetwork *Network) (bool, bool, error) {
	if err := newNetwork.Validate(); err != nil {
		return false, false, err
	}
	if newNetwork.NetID == currentNetwork.NetID {
		hasrangeupdate := newNetwork.AddressRange != currentNetwork.AddressRange
		localrangeupdate := newNetwork.LocalRange != currentNetwork.LocalRange
		if data, err := json.Marshal(newNetwork); err != nil {
			return false, false, err
		} else {
			newNetwork.SetNetworkLastModified()
			err = database.Insert(newNetwork.NetID, string(data), database.NETWORKS_TABLE_NAME)
			return hasrangeupdate, localrangeupdate, err
		}
	}
	// copy values
	return false, false, errors.New("failed to update network " + newNetwork.NetID + ", cannot change netid.")
}
