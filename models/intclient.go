package models

type IntClient struct {
	ClientID             string `json:"clientid" bson:"clientid"`
	PrivateKey           string `json:"privatekey" bson:"privatekey"`
	PublicKey            string `json:"publickey" bson:"publickey"`
	AccessKey            string `json:"accesskey" bson:"accesskey"`
	Address              string `json:"address" bson:"address"`
	Address6             string `json:"address6" bson:"address6"`
	Network              string `json:"network" bson:"network"`
	ServerPublicEndpoint string `json:"serverpublicendpoint" bson:"serverpublicendpoint"`
	ServerAPIPort        string `json:"serverapiport" bson:"serverapiport"`
	ServerPrivateAddress string `json:"serverprivateaddress" bson:"serverprivateaddress"`
	ServerWGPort         string `json:"serverwgport" bson:"serverwgport"`
	ServerKey            string `json:"serverkey" bson:"serverkey"`
	IsServer             string `json:"isserver" bson:"isserver"`
}
