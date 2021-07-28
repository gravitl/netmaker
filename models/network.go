package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	DefaultSaveConfig      string      `json:"defaultsaveconfig" bson:"defaultsaveconfig" validate:"checkyesorno"`
	AccessKeys             []AccessKey `json:"accesskeys" bson:"accesskeys"`
	AllowManualSignUp      string      `json:"allowmanualsignup" bson:"allowmanualsignup" validate:"checkyesorno"`
	IsLocal                string      `json:"islocal" bson:"islocal" validate:"checkyesorno"`
	IsDualStack            string      `json:"isdualstack" bson:"isdualstack" validate:"checkyesorno"`
	IsIPv4                 string      `json:"isipv4" bson:"isipv4" validate:"checkyesorno"`
	IsIPv6                 string      `json:"isipv6" bson:"isipv6" validate:"checkyesorno"`
	IsGRPCHub              string      `json:"isgrpchub" bson:"isgrpchub" validate:"checkyesorno"`
	LocalRange             string      `json:"localrange" bson:"localrange" validate:"omitempty,cidr"`
	DefaultCheckInInterval int32       `json:"checkininterval,omitempty" bson:"checkininterval,omitempty" validate:"omitempty,numeric,min=2,max=100000"`
	DefaultUDPHolePunch    string      `json:"defaultudpholepunch" bson:"defaultudpholepunch" validate:"checkyesorno"`
}

type SaveData struct { // put sensitive fields here
	NetID string `json:"netid" bson:"netid" validate:"required,min=1,max=12,netid_valid"`
}

const STRING_FIELD_TYPE = "string"
const INT64_FIELD_TYPE = "int64"
const INT32_FIELD_TYPE = "int32"
const ACCESS_KEY_TYPE = "[]AccessKey"

var FIELD_TYPES = []string{STRING_FIELD_TYPE, INT64_FIELD_TYPE, INT32_FIELD_TYPE, ACCESS_KEY_TYPE}

var FIELDS = map[string][]string{
	// "id":                  {"ID", "string"},
	"addressrange":        {"AddressRange", STRING_FIELD_TYPE},
	"addressrange6":       {"AddressRange6", STRING_FIELD_TYPE},
	"displayname":         {"DisplayName", STRING_FIELD_TYPE},
	"netid":               {"NetID", STRING_FIELD_TYPE},
	"nodeslastmodified":   {"NodesLastModified", INT64_FIELD_TYPE},
	"networklastmodified": {"NetworkLastModified", INT64_FIELD_TYPE},
	"defaultinterface":    {"DefaultInterface", STRING_FIELD_TYPE},
	"defaultlistenport":   {"DefaultListenPort", INT32_FIELD_TYPE},
	"nodelimit":           {"NodeLimit", INT32_FIELD_TYPE},
	"defaultpostup":       {"DefaultPostUp", STRING_FIELD_TYPE},
	"defaultpostdown":     {"DefaultPostDown", STRING_FIELD_TYPE},
	"keyupdatetimestamp":  {"KeyUpdateTimeStamp", INT64_FIELD_TYPE},
	"defaultkeepalive":    {"DefaultKeepalive", INT32_FIELD_TYPE},
	"defaultsaveconfig":   {"DefaultSaveConfig", STRING_FIELD_TYPE},
	"accesskeys":          {"AccessKeys", ACCESS_KEY_TYPE},
	"allowmanualsignup":   {"AllowManualSignUp", STRING_FIELD_TYPE},
	"islocal":             {"IsLocal", STRING_FIELD_TYPE},
	"isdualstack":         {"IsDualStack", STRING_FIELD_TYPE},
	"isipv4":              {"IsIPv4", STRING_FIELD_TYPE},
	"isipv6":              {"IsIPv6", STRING_FIELD_TYPE},
	"isgrpchub":           {"IsGRPCHub", STRING_FIELD_TYPE},
	"localrange":          {"LocalRange", STRING_FIELD_TYPE},
	"checkininterval":     {"DefaultCheckInInterval", INT32_FIELD_TYPE},
	"defaultudpholepunch": {"DefaultUDPHolePunch", STRING_FIELD_TYPE},
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

func (network *Network) Validate(isUpdate bool) error {
	v := validator.New()
	_ = v.RegisterValidation("netid_valid", func(fl validator.FieldLevel) bool {
		inCharSet := network.NetIDInNetworkCharSet()
		if isUpdate {
			return inCharSet
		}
		isFieldUnique, _ := network.IsNetworkNameUnique()
		return isFieldUnique && inCharSet
	})
	//
	_ = v.RegisterValidation("displayname_valid", func(fl validator.FieldLevel) bool {
		isFieldUnique, _ := network.IsNetworkDisplayNameUnique()
		inCharSet := network.DisplayNameInNetworkCharSet()
		if isUpdate {
			return inCharSet
		}
		return isFieldUnique && inCharSet
	})
	_ = v.RegisterValidation("checkyesorno", func(fl validator.FieldLevel) bool {
		return CheckYesOrNo(fl)
	})
	err := v.Struct(network)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}

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

func (network *Network) CopyValues(newNetwork *Network, fieldName string) {
	reflection := reflect.ValueOf(newNetwork)
	value := reflect.Indirect(reflection).FieldByName(FIELDS[fieldName][0])
	if value.IsValid() && len(FIELDS[fieldName]) == 2 {
		fieldData := FIELDS[fieldName]
		for _, indexVal := range FIELD_TYPES {
			if indexVal == fieldData[1] {
				currentReflection := reflect.ValueOf(network)
				reflect.Indirect(currentReflection).FieldByName(FIELDS[fieldName][0]).Set(value)
			}
		}
	}
}

func (currentNetwork *Network) Update(newNetwork *Network) (bool, bool, error) {
	if err := newNetwork.Validate(true); err != nil {
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

func (network *Network) SetNetworkNodesLastModified() error {

        timestamp := time.Now().Unix()

        network.NodesLastModified = timestamp
        data, err := json.Marshal(&network)
        if err != nil {
                return err
        }
        err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME)
        if err != nil {
                return err
        }
        return nil
}

func GetNetwork(networkname string) (Network, error) {

        var network Network
        networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
        if err != nil {
                return network, err
        }
        if err = json.Unmarshal([]byte(networkData), &network); err != nil {
                return Network{}, err
        }
        return network, nil
}
