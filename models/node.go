package models

import (
	"encoding/json"
	"math/rand"
	"net"
	"time"

	"github.com/gravitl/netmaker/database"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

//node struct
type Node struct {
	ID                  primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Address             string             `json:"address" bson:"address" validate:"omitempty,ipv4"`
	Address6            string             `json:"address6" bson:"address6" validate:"omitempty,ipv6"`
	LocalAddress        string             `json:"localaddress" bson:"localaddress" validate:"omitempty,ip"`
	Name                string             `json:"name" bson:"name" validate:"omitempty,max=12,in_charset"`
	ListenPort          int32              `json:"listenport" bson:"listenport" validate:"omitempty,numeric,min=1024,max=65535"`
	PublicKey           string             `json:"publickey" bson:"publickey" validate:"required,base64"`
	Endpoint            string             `json:"endpoint" bson:"endpoint" validate:"required,ip"`
	PostUp              string             `json:"postup" bson:"postup"`
	PostDown            string             `json:"postdown" bson:"postdown"`
	AllowedIPs          []string           `json:"allowedips" bson:"allowedips"`
	PersistentKeepalive int32              `json:"persistentkeepalive" bson:"persistentkeepalive" validate:"omitempty,numeric,max=1000"`
	SaveConfig          *bool              `json:"saveconfig" bson:"saveconfig"`
	AccessKey           string             `json:"accesskey" bson:"accesskey"`
	Interface           string             `json:"interface" bson:"interface"`
	LastModified        int64              `json:"lastmodified" bson:"lastmodified"`
	KeyUpdateTimeStamp  int64              `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
	ExpirationDateTime  int64              `json:"expdatetime" bson:"expdatetime"`
	LastPeerUpdate      int64              `json:"lastpeerupdate" bson:"lastpeerupdate"`
	LastCheckIn         int64              `json:"lastcheckin" bson:"lastcheckin"`
	MacAddress          string             `json:"macaddress" bson:"macaddress" validate:"required,mac,macaddress_unique"`
	CheckInInterval     int32              `json:"checkininterval" bson:"checkininterval"`
	Password            string             `json:"password" bson:"password" validate:"required,min=6"`
	Network             string             `json:"network" bson:"network" validate:"network_exists"`
	IsPending           bool               `json:"ispending" bson:"ispending"`
	IsEgressGateway     bool               `json:"isegressgateway" bson:"isegressgateway"`
	IsIngressGateway    bool               `json:"isingressgateway" bson:"isingressgateway"`
	EgressGatewayRanges []string           `json:"egressgatewayranges" bson:"egressgatewayranges"`
	IngressGatewayRange string             `json:"ingressgatewayrange" bson:"ingressgatewayrange"`
	PostChanges         string             `json:"postchanges" bson:"postchanges"`
	StaticIP            string             `json:"staticip" bson:"staticip"`
	StaticPubKey        string             `json:"staticpubkey" bson:"staticpubkey"`
}

//node update struct --- only validations are different
type NodeUpdate struct {
	ID                  primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Address             string             `json:"address" bson:"address" validate:"omitempty,ip"`
	Address6            string             `json:"address6" bson:"address6" validate:"omitempty,ipv6"`
	LocalAddress        string             `json:"localaddress" bson:"localaddress" validate:"omitempty,ip"`
	Name                string             `json:"name" bson:"name" validate:"omitempty,max=12,in_charset"`
	ListenPort          int32              `json:"listenport" bson:"listenport" validate:"omitempty,numeric,min=1024,max=65535"`
	PublicKey           string             `json:"publickey" bson:"publickey" validate:"omitempty,base64"`
	Endpoint            string             `json:"endpoint" bson:"endpoint" validate:"omitempty,ip"`
	PostUp              string             `json:"postup" bson:"postup"`
	PostDown            string             `json:"postdown" bson:"postdown"`
	AllowedIPs          []string           `json:"allowedips" bson:"allowedips"`
	PersistentKeepalive int32              `json:"persistentkeepalive" bson:"persistentkeepalive" validate:"omitempty,numeric,max=1000"`
	SaveConfig          *bool              `json:"saveconfig" bson:"saveconfig"`
	AccessKey           string             `json:"accesskey" bson:"accesskey"`
	Interface           string             `json:"interface" bson:"interface"`
	LastModified        int64              `json:"lastmodified" bson:"lastmodified"`
	KeyUpdateTimeStamp  int64              `json:"keyupdatetimestamp" bson:"keyupdatetimestamp"`
	ExpirationDateTime  int64              `json:"expdatetime" bson:"expdatetime"`
	LastPeerUpdate      int64              `json:"lastpeerupdate" bson:"lastpeerupdate"`
	LastCheckIn         int64              `json:"lastcheckin" bson:"lastcheckin"`
	MacAddress          string             `json:"macaddress" bson:"macaddress" validate:"required,mac"`
	CheckInInterval     int32              `json:"checkininterval" bson:"checkininterval"`
	Password            string             `json:"password" bson:"password" validate:"omitempty,min=5"`
	Network             string             `json:"network" bson:"network" validate:"network_exists"`
	IsPending           bool               `json:"ispending" bson:"ispending"`
	IsIngressGateway    bool               `json:"isingressgateway" bson:"isingressgateway"`
	IsEgressGateway     bool               `json:"isegressgateway" bson:"isegressgateway"`
	IngressGatewayRange string             `json:"ingressgatewayrange" bson:"ingressgatewayrange"`
	EgressGatewayRanges []string           `json:"egressgatewayranges" bson:"egressgatewayranges"`
	PostChanges         string             `json:"postchanges" bson:"postchanges"`
	StaticIP            string             `json:"staticip" bson:"staticip"`
	StaticPubKey        string             `json:"staticpubkey" bson:"staticpubkey"`
}

//TODO:
//Not sure if below two methods are necessary. May want to revisit
func (node *Node) SetLastModified() {
	node.LastModified = time.Now().Unix()
}

func (node *Node) SetLastCheckIn() {
	node.LastCheckIn = time.Now().Unix()
}

func (node *Node) SetLastPeerUpdate() {
	node.LastPeerUpdate = time.Now().Unix()
}

func (node *Node) SetExpirationDateTime() {
	node.ExpirationDateTime = time.Unix(33174902665, 0).Unix()
}

func (node *Node) SetDefaultName() {
	if node.Name == "" {
		nodeid := StringWithCharset(5, charset)
		nodename := "node-" + nodeid
		node.Name = nodename
	}
}

func (node *Node) GetNetwork() (Network, error) {

	var network Network
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, node.Network)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return Network{}, err
	}
	return network, nil
}

//TODO: I dont know why this exists
//This should exist on the node.go struct. I'm sure there was a reason?
func (node *Node) SetDefaults() {

	//TODO: Maybe I should make Network a part of the node struct. Then we can just query the Network object for stuff.
	parentNetwork, _ := node.GetNetwork()

	node.ExpirationDateTime = time.Unix(33174902665, 0).Unix()

	if node.ListenPort == 0 {
		node.ListenPort = parentNetwork.DefaultListenPort
	}
	if node.PostDown == "" {
		//Empty because we dont set it
		//may want to set it to something in the future
	}
	//TODO: This is dumb and doesn't work
	//Need to change
	if node.SaveConfig == nil {
		if parentNetwork.DefaultSaveConfig != "" {
			defaultsave := parentNetwork.DefaultSaveConfig == "yes"
			node.SaveConfig = &defaultsave
		}
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
	if node.StaticIP == "" {
		node.StaticIP = "no"
	}
	if node.StaticPubKey == "" {
		node.StaticPubKey = "no"
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
