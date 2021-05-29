package models

type IntClient struct {
        ClientID       string             `json:"clientid" bson:"clientid"`
	PrivateKey     string             `json:"privatekey" bson:"privatekey"`
	PublicKey      string             `json:"publickey" bson:"publickey"`
	AccessKey      string             `json:"accesskey" bson:"accesskey"`
	Address        string             `json:"address" bson:"address"`
	Address6        string             `json:"address6" bson:"address6"`
	Network        string             `json:"network" bson:"network"`
	ServerEndpoint  string             `json:"serverendpoint" bson:"serverendpoint"`
        ServerAPIEndpoint  string             `json:"serverapiendpoint" bson:"serverapiendpoint"`
	ServerAddress  string             `json:"serveraddress" bson:"serveraddress"`
	ServerPort     string             `json:"serverport" bson:"serverport"`
	ServerKey      string             `json:"serverkey" bson:"serverkey"`
	IsServer       string             `json:"isserver" bson:"isserver"`
}
