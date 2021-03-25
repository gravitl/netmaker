//TODO:  Either add a returnGroup and returnKey, or delete this
package models

type ReturnNode struct {
	Address	string `json:"address" bson:"address"`
	Name	string `json:"name" bson:"name"`
	MacAddress string `json:"macaddress" bson:"macaddress"`
	LastCheckIn int64 `json:"lastcheckin" bson:"lastcheckin"`
	LastModified int64 `json:"lastmodified" bson:"lastmodified"`
	LastPeerUpdate int64 `json:"lastpeerupdate" bson:"lastpeerupdate"`
	ListenPort	int32 `json:"listenport" bson:"listenport"`
	PublicKey	string `json:"publickey" bson:"publickey" validate:"base64"`
	Endpoint	string `json:"endpoint" bson:"endpoint" validate:"required,ipv4"`
	PostUp	string `json:"postup" bson:"postup"`
	PreUp	string `json:"preup" bson:"preup"`
	PersistentKeepalive int32 `json:"persistentkeepalive" bson:"persistentkeepalive"`
	SaveConfig	*bool `json:"saveconfig" bson:"saveconfig"`
	Interface	string `json:"interface" bson:"interface"`
	Group	string `json:"group" bson:"group"`
	IsPending	*bool `json:"ispending" bson:"ispending"`
}
