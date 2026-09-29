package models

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/gravitl/netmaker/schema"
)

type ExtClient = schema.ExtClient

// CustomExtClient - struct for CustomExtClient params
type CustomExtClient struct {
	ClientID                   string              `json:"clientid,omitempty"`
	PublicKey                  string              `json:"publickey,omitempty"`
	DNS                        string              `json:"dns,omitempty"`
	ExtraAllowedIPs            []string            `json:"extraallowedips,omitempty"`
	Enabled                    bool                `json:"enabled,omitempty"`
	DeniedACLs                 map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID       string              `json:"remote_access_client_id"` // unique ID (MAC address) of RAC machine
	PostUp                     string              `json:"postup" bson:"postup" validate:"max=1024"`
	PostDown                   string              `json:"postdown" bson:"postdown" validate:"max=1024"`
	Tags                       ExtClientTags       `json:"tags,omitempty"`
	DeviceID                   string              `json:"device_id"`
	DeviceName                 string              `json:"device_name"`
	IsAlreadyConnectedToInetGw bool                `json:"is_already_connected_to_inet_gw"`
	PublicEndpoint             string              `json:"public_endpoint"`
	OS                         string              `json:"os"`
	OSFamily                   string              `json:"os_family" yaml:"os_family"`
	OSVersion                  string              `json:"os_version"                      yaml:"os_version"`
	KernelVersion              string              `json:"kernel_version" yaml:"kernel_version"`
	ClientVersion              string              `json:"client_version"`
	Country                    string              `json:"country"`
	Location                   string              `json:"location"` //format: lat,long
	UseInternetEgress          *bool               `json:"use_internet_egress,omitempty"`
	SelectedInternetEgressID   string              `json:"selected_internet_egress_id,omitempty"`
}

// ExtClientTags is the tag set stored on an extclient. JSON requests send a list of tag IDs.
type ExtClientTags map[TagID]struct{}

func (t *ExtClientTags) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '[' {
		var ids []string
		if err := json.Unmarshal(data, &ids); err != nil {
			return err
		}
		set := make(ExtClientTags, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			set[TagID(id)] = struct{}{}
		}
		*t = set
		return nil
	}
	var set map[TagID]struct{}
	if err := json.Unmarshal(data, &set); err != nil {
		return err
	}
	*t = set
	return nil
}

func (t ExtClientTags) Map() map[TagID]struct{} {
	if t == nil {
		return nil
	}
	return map[TagID]struct{}(t)
}

func ConvertToStaticNode(ext ExtClient) Node {
	if ext.Tags == nil {
		ext.Tags = make(map[TagID]struct{})
	}
	return Node{
		CommonNode: CommonNode{
			Network:  ext.Network,
			Address:  ext.AddressIPNet4(),
			Address6: ext.AddressIPNet6(),
		},
		Tags:                               ext.Tags,
		IsStatic:                           true,
		StaticNode:                         ext,
		IsUserNode:                         ext.RemoteAccessClientID != "" || ext.DeviceID != "",
		Mutex:                              ext.Mutex,
		CountryCode:                        ext.Country,
		Location:                           ext.Location,
		PostureChecksViolations:            ext.PostureChecksViolations,
		PostureCheckViolationSeverityLevel: ext.PostureCheckVolationSeverityLevel,
		LastEvaluatedAt:                    ext.LastEvaluatedAt,
	}
}
