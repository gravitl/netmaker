package models

import "sync"

// ExtClient - struct for external clients
type ExtClient struct {
	ClientID               string              `json:"clientid" bson:"clientid"`
	PrivateKey             string              `json:"privatekey" bson:"privatekey"`
	PublicKey              string              `json:"publickey" bson:"publickey"`
	Network                string              `json:"network" bson:"network"`
	DNS                    string              `json:"dns" bson:"dns"`
	Address                string              `json:"address" bson:"address"`
	Address6               string              `json:"address6" bson:"address6"`
	ExtraAllowedIPs        []string            `json:"extraallowedips" bson:"extraallowedips"`
	AllowedIPs             []string            `json:"allowed_ips"`
	IngressGatewayID       string              `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint string              `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
	LastModified           int64               `json:"lastmodified" bson:"lastmodified" swaggertype:"primitive,integer" format:"int64"`
	Enabled                bool                `json:"enabled" bson:"enabled"`
	OwnerID                string              `json:"ownerid" bson:"ownerid"`
	DeniedACLs             map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID   string              `json:"remote_access_client_id"` // unique ID (MAC address) of RAC machine
	PostUp                 string              `json:"postup" bson:"postup"`
	PostDown               string              `json:"postdown" bson:"postdown"`
	Tags                   map[TagID]struct{}  `json:"tags"`
	Os                     string              `json:"os"`
	DeviceName             string              `json:"device_name"`
	PublicEndpoint         string              `json:"public_endpoint"`
	Country                string              `json:"country"`
	Location               string              `json:"location"` //format: lat,long
	Mutex                  *sync.Mutex         `json:"-"`
}

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
	Tags                       map[TagID]struct{}  `json:"tags"`
	Os                         string              `json:"os"`
	DeviceName                 string              `json:"device_name"`
	IsAlreadyConnectedToInetGw bool                `json:"is_already_connected_to_inet_gw"`
	PublicEndpoint             string              `json:"public_endpoint"`
	Country                    string              `json:"country"`
	Location                   string              `json:"location"` //format: lat,long
}

func (ext *ExtClient) ConvertToStaticNode() Node {
	if ext.Tags == nil {
		ext.Tags = make(map[TagID]struct{})
	}
	return Node{
		CommonNode: CommonNode{
			Network:  ext.Network,
			Address:  ext.AddressIPNet4(),
			Address6: ext.AddressIPNet6(),
		},
		Tags:       ext.Tags,
		IsStatic:   true,
		StaticNode: *ext,
		IsUserNode: ext.RemoteAccessClientID != "",
		Mutex:      ext.Mutex,
	}
}
