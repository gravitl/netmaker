package models

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
	IngressGatewayID       string              `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint string              `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
	LastModified           int64               `json:"lastmodified" bson:"lastmodified"`
	Enabled                bool                `json:"enabled" bson:"enabled"`
	OwnerID                string              `json:"ownerid" bson:"ownerid"`
	DeniedACLs             map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID   string              `json:"remote_access_client_id"`
}

// CustomExtClient - struct for CustomExtClient params
type CustomExtClient struct {
	ClientID             string              `json:"clientid,omitempty"`
	PublicKey            string              `json:"publickey,omitempty"`
	DNS                  string              `json:"dns,omitempty"`
	ExtraAllowedIPs      []string            `json:"extraallowedips,omitempty"`
	Enabled              bool                `json:"enabled,omitempty"`
	DeniedACLs           map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID string              `json:"remote_access_client_id"`
}
