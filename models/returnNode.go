//TODO:  Either add a returnNetwork and returnKey, or delete this
package models

type ReturnNode struct {
	Address	string `json:"address" bson:"address"`
	Address6 string `json:"address6" bson:"address6"`
	Name	string `json:"name" bson:"name"`
	MacAddress string `json:"macaddress" bson:"macaddress"`
	LastCheckIn int64 `json:"lastcheckin" bson:"lastcheckin"`
	LastModified int64 `json:"lastmodified" bson:"lastmodified"`
	LastPeerUpdate int64 `json:"lastpeerupdate" bson:"lastpeerupdate"`
	ListenPort	int32 `json:"listenport" bson:"listenport"`
	PublicKey	string `json:"publickey" bson:"publickey" validate:"base64"`
	Endpoint	string `json:"endpoint" bson:"endpoint" validate:"required,ipv4"`
	PostUp	string `json:"postup" bson:"postup"`
	PostDown	string `json:"postdown" bson:"postdown"`
	PersistentKeepalive int32 `json:"persistentkeepalive" bson:"persistentkeepalive"`
	SaveConfig	*bool `json:"saveconfig" bson:"saveconfig"`
	Interface	string `json:"interface" bson:"interface"`
	Network	string `json:"network" bson:"network"`
	IsPending	*bool `json:"ispending" bson:"ispending"`
	IsGateway	*bool `json:"isgateway" bson:"isgateway"`
	GatewayRange	string `json:"gatewayrange" bson:"gatewayrange"`
        LocalAddress    string `json:"localaddress" bson:"localaddress" validate:"localaddress_check"`
        ExpirationDateTime      int64 `json:"expdatetime" bson:"expdatetime"`
}
