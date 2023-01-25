package models

// ExtClient - struct for external clients
type ExtClient struct {
	ClientID               string `json:"clientid" bson:"clientid"`
	Description            string `json:"description" bson:"description"`
	PrivateKey             string `json:"privatekey" bson:"privatekey"`
	PublicKey              string `json:"publickey" bson:"publickey"`
	Network                string `json:"network" bson:"network"`
	Address                string `json:"address" bson:"address"`
	Address6               string `json:"address6" bson:"address6"`
	IngressGatewayID       string `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint string `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
	LastModified           int64  `json:"lastmodified" bson:"lastmodified"`
	Enabled                bool   `json:"enabled" bson:"enabled"`
	OwnerID                string `json:"ownerid" bson:"ownerid"`
}
