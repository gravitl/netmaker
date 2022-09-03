package models

import (
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const PLACEHOLDER_KEY_TEXT = "ACCESS_KEY"
const PLACEHOLDER_TOKEN_TEXT = "ACCESS_TOKEN"

// CustomExtClient - struct for CustomExtClient params
type CustomExtClient struct {
	ClientID string `json:"clientid"`
}

// AuthParams - struct for auth params
type AuthParams struct {
	MacAddress string `json:"macaddress"`
	ID         string `json:"id"`
	Password   string `json:"password"`
}

// User struct - struct for Users
type User struct {
	UserName string   `json:"username" bson:"username" validate:"min=3,max=40,in_charset|email"`
	Password string   `json:"password" bson:"password" validate:"required,min=5"`
	Networks []string `json:"networks" bson:"networks"`
	IsAdmin  bool     `json:"isadmin" bson:"isadmin"`
}

// ReturnUser - return user struct
type ReturnUser struct {
	UserName string   `json:"username" bson:"username"`
	Networks []string `json:"networks" bson:"networks"`
	IsAdmin  bool     `json:"isadmin" bson:"isadmin"`
}

// UserAuthParams - user auth params struct
type UserAuthParams struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// UserClaims - user claims struct
type UserClaims struct {
	IsAdmin  bool
	UserName string
	Networks []string
	jwt.RegisteredClaims
}

// SuccessfulUserLoginResponse - successlogin struct
type SuccessfulUserLoginResponse struct {
	UserName  string
	AuthToken string
}

// Claims is  a struct that will be encoded to a JWT.
// jwt.StandardClaims is an embedded type to provide expiry time
type Claims struct {
	ID         string
	MacAddress string
	Network    string
	jwt.RegisteredClaims
}

// SuccessfulLoginResponse is struct to send the request response
type SuccessfulLoginResponse struct {
	ID        string
	AuthToken string
}

// ErrorResponse is struct for error
type ErrorResponse struct {
	Code    int
	Message string
}

// NodeAuth - struct for node auth
type NodeAuth struct {
	Network    string
	Password   string
	MacAddress string // Depricated
	ID         string
}

// SuccessResponse is struct for sending error message with code.
type SuccessResponse struct {
	Code     int
	Message  string
	Response interface{}
}

// AccessKey - access key struct
type AccessKey struct {
	Name         string `json:"name" bson:"name" validate:"omitempty,max=20"`
	Value        string `json:"value" bson:"value" validate:"omitempty,alphanum,max=16"`
	AccessString string `json:"accessstring" bson:"accessstring"`
	Uses         int    `json:"uses" bson:"uses" validate:"numeric,min=0"`
}

// DisplayKey - what is displayed for key
type DisplayKey struct {
	Name string `json:"name" bson:"name"`
	Uses int    `json:"uses" bson:"uses"`
}

// GlobalConfig - global config
type GlobalConfig struct {
	Name string `json:"name" bson:"name"`
}

// CheckInResponse - checkin response
type CheckInResponse struct {
	Success          bool   `json:"success" bson:"success"`
	NeedPeerUpdate   bool   `json:"needpeerupdate" bson:"needpeerupdate"`
	NeedConfigUpdate bool   `json:"needconfigupdate" bson:"needconfigupdate"`
	NeedKeyUpdate    bool   `json:"needkeyupdate" bson:"needkeyupdate"`
	NeedDelete       bool   `json:"needdelete" bson:"needdelete"`
	NodeMessage      string `json:"nodemessage" bson:"nodemessage"`
	IsPending        bool   `json:"ispending" bson:"ispending"`
}

// PeersResponse - peers response
type PeersResponse struct {
	PublicKey           string `json:"publickey" bson:"publickey"`
	Endpoint            string `json:"endpoint" bson:"endpoint"`
	Address             string `json:"address" bson:"address"`
	Address6            string `json:"address6" bson:"address6"`
	LocalAddress        string `json:"localaddress" bson:"localaddress"`
	LocalListenPort     int32  `json:"locallistenport" bson:"locallistenport"`
	IsEgressGateway     string `json:"isegressgateway" bson:"isegressgateway"`
	EgressGatewayRanges string `json:"egressgatewayrange" bson:"egressgatewayrange"`
	ListenPort          int32  `json:"listenport" bson:"listenport"`
	KeepAlive           int32  `json:"persistentkeepalive" bson:"persistentkeepalive"`
}

// ExtPeersResponse - ext peers response
type ExtPeersResponse struct {
	PublicKey       string `json:"publickey" bson:"publickey"`
	Endpoint        string `json:"endpoint" bson:"endpoint"`
	Address         string `json:"address" bson:"address"`
	Address6        string `json:"address6" bson:"address6"`
	LocalAddress    string `json:"localaddress" bson:"localaddress"`
	LocalListenPort int32  `json:"locallistenport" bson:"locallistenport"`
	ListenPort      int32  `json:"listenport" bson:"listenport"`
	KeepAlive       int32  `json:"persistentkeepalive" bson:"persistentkeepalive"`
}

// EgressGatewayRequest - egress gateway request
type EgressGatewayRequest struct {
	NodeID     string   `json:"nodeid" bson:"nodeid"`
	NetID      string   `json:"netid" bson:"netid"`
	NatEnabled string   `json:"natenabled" bson:"natenabled"`
	Ranges     []string `json:"ranges" bson:"ranges"`
	Interface  string   `json:"interface" bson:"interface"`
	PostUp     string   `json:"postup" bson:"postup"`
	PostDown   string   `json:"postdown" bson:"postdown"`
}

// RelayRequest - relay request struct
type RelayRequest struct {
	NodeID     string   `json:"nodeid" bson:"nodeid"`
	NetID      string   `json:"netid" bson:"netid"`
	RelayAddrs []string `json:"relayaddrs" bson:"relayaddrs"`
}

// ServerUpdateData - contains data to configure server
// and if it should set peers
type ServerUpdateData struct {
	UpdatePeers bool `json:"updatepeers" bson:"updatepeers"`
	Node        Node `json:"servernode" bson:"servernode"`
}

// Telemetry - contains UUID of the server and timestamp of last send to posthog
// also contains assymetrical encryption pub/priv keys for any server traffic
type Telemetry struct {
	UUID           string `json:"uuid" bson:"uuid"`
	LastSend       int64  `json:"lastsend" bson:"lastsend"`
	TrafficKeyPriv []byte `json:"traffickeypriv" bson:"traffickeypriv"`
	TrafficKeyPub  []byte `json:"traffickeypub" bson:"traffickeypub"`
}

// ServerAddr - to pass to clients to tell server addresses and if it's the leader or not
type ServerAddr struct {
	IsLeader bool   `json:"isleader" bson:"isleader" yaml:"isleader"`
	Address  string `json:"address" bson:"address" yaml:"address"`
}

// TrafficKeys - struct to hold public keys
type TrafficKeys struct {
	Mine   []byte `json:"mine" bson:"mine" yaml:"mine"`
	Server []byte `json:"server" bson:"server" yaml:"server"`
}

// NodeGet - struct for a single node get response
type NodeGet struct {
	Node         Node                 `json:"node" bson:"node" yaml:"node"`
	Peers        []wgtypes.PeerConfig `json:"peers" bson:"peers" yaml:"peers"`
	ServerConfig ServerConfig         `json:"serverconfig" bson:"serverconfig" yaml:"serverconfig"`
}

// ServerConfig - struct for dealing with the server information for a netclient
type ServerConfig struct {
	CoreDNSAddr string `yaml:"corednsaddr"`
	API         string `yaml:"api"`
	APIPort     string `yaml:"apiport"`
	ClientMode  string `yaml:"clientmode"`
	DNSMode     string `yaml:"dnsmode"`
	Version     string `yaml:"version"`
	MQPort      string `yaml:"mqport"`
	Server      string `yaml:"server"`
}

// User.NameInCharset - returns if name is in charset below or not
func (user *User) NameInCharSet() bool {
	charset := "abcdefghijklmnopqrstuvwxyz1234567890-."
	for _, char := range user.UserName {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}
