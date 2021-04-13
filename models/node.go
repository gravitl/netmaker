package models

import (
  "go.mongodb.org/mongo-driver/bson/primitive"
  "github.com/gravitl/netmaker/mongoconn"
  "math/rand"
  "time"
  "net"
  "context"
  "go.mongodb.org/mongo-driver/bson"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
  rand.NewSource(time.Now().UnixNano()))

//node struct
type Node struct {
	ID	primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Address	string `json:"address" bson:"address"`
	LocalAddress	string `json:"localaddress" bson:"localaddress" validate:"localaddress_check"`
	Name	string `json:"name" bson:"name" validate:"omitempty,name_valid,max=12"`
	ListenPort	int32 `json:"listenport" bson:"listenport" validate:"omitempty,numeric,min=1024,max=65535"`
	PublicKey	string `json:"publickey" bson:"publickey" validate:"pubkey_check"`
	Endpoint	string `json:"endpoint" bson:"endpoint" validate:"endpoint_check"`
	PostUp	string `json:"postup" bson:"postup"`
	PreUp	string `json:"preup" bson:"preup"`
	AllowedIPs	string `json:"allowedips" bson:"allowedips"`
	PersistentKeepalive int32 `json:"persistentkeepalive" bson:"persistentkeepalive" validate: "omitempty,numeric,max=1000"`
	SaveConfig	*bool `json:"saveconfig" bson:"saveconfig"`
	AccessKey	string `json:"accesskey" bson:"accesskey"`
	Interface	string `json:"interface" bson:"interface"`
	LastModified	int64 `json:"lastmodified" bson:"lastmodified"`
	KeyUpdateTimeStamp	int64 `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
	ExpirationDateTime	int64 `json:"expdatetime" bson:"expdatetime"`
	LastPeerUpdate	int64 `json:"lastpeerupdate" bson:"lastpeerupdate"`
	LastCheckIn	int64 `json:"lastcheckin" bson:"lastcheckin"`
	MacAddress	string `json:"macaddress" bson:"macaddress" validate:"required,macaddress_valid,macaddress_unique"`
	CheckInInterval	int32 `json:"checkininterval" bson:"checkininterval"`
	Password	string `json:"password" bson:"password" validate:"password_check"`
	Network	string `json:"network" bson:"network" validate:"network_exists"`
	IsPending bool `json:"ispending" bson:"ispending"`
	IsGateway bool `json:"isgateway" bson:"isgateway"`
	GatewayRange string `json:"gatewayrange" bson:"gatewayrange"`
	PostChanges string `json:"postchanges" bson:"postchanges"`
}


//TODO: Contains a fatal error return. Need to change
//Used in contexts where it's not the Parent network.
func(node *Node) GetNetwork() (Network, error){

        var network Network

        collection := mongoconn.NetworkDB
        //collection := mongoconn.Client.Database("netmaker").Collection("networks")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"netid": node.Network}
        err := collection.FindOne(ctx, filter).Decode(&network)

        defer cancel()

        if err != nil {
                //log.Fatal(err)
                return network, err
        }

        return network, err
}


//TODO:
//Not sure if below two methods are necessary. May want to revisit
func(node *Node) SetLastModified(){
	node.LastModified = time.Now().Unix()
}

func(node *Node) SetLastCheckIn(){
        node.LastCheckIn = time.Now().Unix()
}

func(node *Node) SetLastPeerUpdate(){
        node.LastPeerUpdate = time.Now().Unix()
}

func(node *Node) SetExpirationDateTime(){
        node.ExpirationDateTime = time.Unix(33174902665, 0).Unix()
}


func(node *Node) SetDefaultName(){
    if node.Name == "" {
        nodeid := StringWithCharset(5, charset)
        nodename := "node-" + nodeid
        node.Name = nodename
    }
}

//TODO: I dont know why this exists
//This should exist on the node.go struct. I'm sure there was a reason?
func(node *Node) SetDefaults() {

    //TODO: Maybe I should make Network a part of the node struct. Then we can just query the Network object for stuff.
    parentNetwork, _ := node.GetNetwork()

    node.ExpirationDateTime = time.Unix(33174902665, 0).Unix()

    if node.ListenPort == 0 {
        node.ListenPort = parentNetwork.DefaultListenPort
    }
    if node.PreUp == "" {
        //Empty because we dont set it
        //may want to set it to something in the future
    }
    //TODO: This is dumb and doesn't work
    //Need to change
    if node.SaveConfig == nil {
        defaultsave := *parentNetwork.DefaultSaveConfig
        node.SaveConfig = &defaultsave
    }
    if node.Interface == "" {
        node.Interface = parentNetwork.DefaultInterface
    }
    if node.PersistentKeepalive == 0 {
        node.PersistentKeepalive = parentNetwork.DefaultKeepalive
    }
    if node.PostUp == "" {
            postup := parentNetwork.DefaultPostUp
        node.PostUp = postup
    }
    node.CheckInInterval = parentNetwork.DefaultCheckInInterval

}

func StringWithCharset(length int, charset string) string {
  b := make([]byte, length)
  for i := range b {
    b[i] = charset[seededRand.Intn(len(charset))]
  }
  return string(b)
}

//Check for valid IPv4 address
//Note: We dont handle IPv6 AT ALL!!!!! This definitely is needed at some point
//But for iteration 1, lets just stick to IPv4. Keep it simple stupid.
func IsIpv4Net(host string) bool {
   return net.ParseIP(host) != nil
}

