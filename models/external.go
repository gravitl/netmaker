package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

//External Struct
//At  some point, need to replace all instances of Name with something else like  Identifier
type External struct {
	ID           primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ClientID       		string		`json:"clientid" bson:"clientid"`
	PrivateKey 			string		`json:"privatekey" bson:"privatekey"`
	PublicKey  			string		`json:"publickey" bson:"publickey"`
	Network				string		`json:"network" bson:"network"`
	Address         	string		`json:"address" bson:"address"`
	LastModified    	string		`json:"lastmodified" bson:"lastmodified"`
	IngressGateway		string		`json:"ingressgateway" bson:"ingressgateway"`
}
