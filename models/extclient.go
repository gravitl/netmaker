package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)
//What the client needs to get
/*

[Interface]
# The address their computer will use on the network
Address = 10.0.0.8/32 # The Address they'll use on the network
PrivateKey = XXXXXXXXXXXXXXXX # The private key they'll use


# All of this info can come from the node!!
[Peer]
# Ingress Gateway's wireguard public key
PublicKey = CcZHeaO08z55/x3FXdsSGmOQvZG32SvHlrwHnsWlGTs=

# Public IP address of the Ingress Gateway
# Use the floating IP address if you created one for your VPN server
Endpoint = 123.123.123.123:51820

# 10.0.0.0/24 is the VPN sub

*/


// External Struct
// == BACKEND FIELDS ==
// PrivateKey, PublicKey, Address (Private), LastModified, IngressEndpoint
// == FRONTEND FIELDS ==
// ClientID, Network, IngressGateway
type ExtClient struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ClientID       string             `json:"clientid" bson:"clientid"`
	Description       string             `json:"description" bson:"description"`
	PrivateKey     string             `json:"privatekey" bson:"privatekey"`
	PublicKey      string             `json:"publickey" bson:"publickey"`
	Network        string             `json:"network" bson:"network"`
	Address        string             `json:"address" bson:"address"`
	LastModified   int64              `json:"lastmodified" bson:"lastmodified"`
	IngressGatewayID string             `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint string             `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
}
