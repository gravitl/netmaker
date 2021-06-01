package models

type IntClient struct {
	ClientID       string             `json:"clientid" bson:"clientid"`
	PrivateKey     string             `json:"privatekey" bson:"privatekey"`
	PublicKey      string             `json:"publickey" bson:"publickey"`
	AccessKey      string             `json:"accesskey" bson:"accesskey"`
	Address        string             `json:"address" bson:"address"`
	Address6       string             `json:"address6" bson:"address6"`
	Network        string             `json:"network" bson:"network"`
	ServerPublicEndpoint  string `json:"serverwgendpoint" bson:"serverwgendpoint"`
	ServerAPIPort  string      `json:"serverapiendpoint" bson:"serverapiendpoint"`
	ServerPrivateAddress  string       `json:"serveraddress" bson:"serveraddress"`
	ServerWGPort     string             `json:"serverport" bson:"serverport"`
	ServerGRPCPort     string             `json:"serverport" bson:"serverport"`
	ServerKey      string             `json:"serverkey" bson:"serverkey"`
	IsServer       string             `json:"isserver" bson:"isserver"`
}
