package models

import jwt "github.com/golang-jwt/jwt/v4"

type AuthParams struct {
	MacAddress string `json:"macaddress"`
	Password   string `json:"password"`
}

type User struct {
	UserName string   `json:"username" bson:"username" validate:"min=3,max=40,regexp=^(([a-zA-Z,\-,\.]*)|([A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,4})){3,40}$"`
	Password string   `json:"password" bson:"password" validate:"required,min=5"`
	Networks []string `json:"networks" bson:"networks"`
	IsAdmin  bool     `json:"isadmin" bson:"isadmin"`
}

type ReturnUser struct {
	UserName string   `json:"username" bson:"username" validate:"min=3,max=40,regexp=^(([a-zA-Z,\-,\.]*)|([A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,4})){3,40}$"`
	Networks []string `json:"networks" bson:"networks"`
	IsAdmin  bool     `json:"isadmin" bson:"isadmin"`
}

type UserAuthParams struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type UserClaims struct {
	IsAdmin  bool
	UserName string
	Networks []string
	jwt.StandardClaims
}

type SuccessfulUserLoginResponse struct {
	UserName  string
	AuthToken string
}

// Claims is  a struct that will be encoded to a JWT.
// jwt.StandardClaims is an embedded type to provide expiry time
type Claims struct {
	Network    string
	MacAddress string
	jwt.StandardClaims
}

// SuccessfulLoginResponse is struct to send the request response
type SuccessfulLoginResponse struct {
	MacAddress string
	AuthToken  string
}

type ErrorResponse struct {
	Code    int
	Message string
}

type NodeAuth struct {
	Network    string
	Password   string
	MacAddress string
}

// SuccessResponse is struct for sending error message with code.
type SuccessResponse struct {
	Code     int
	Message  string
	Response interface{}
}

type AccessKey struct {
	Name         string `json:"name" bson:"name" validate:"omitempty,max=20"`
	Value        string `json:"value" bson:"value" validate:"omitempty,alphanum,max=16"`
	AccessString string `json:"accessstring" bson:"accessstring"`
	Uses         int    `json:"uses" bson:"uses"`
}

type DisplayKey struct {
	Name string `json:"name" bson:"name"`
	Uses int    `json:"uses" bson:"uses"`
}

type GlobalConfig struct {
	Name       string `json:"name" bson:"name"`
	PortGRPC   string `json:"portgrpc" bson:"portgrpc"`
	ServerGRPC string `json:"servergrpc" bson:"servergrpc"`
}

type CheckInResponse struct {
	Success          bool   `json:"success" bson:"success"`
	NeedPeerUpdate   bool   `json:"needpeerupdate" bson:"needpeerupdate"`
	NeedConfigUpdate bool   `json:"needconfigupdate" bson:"needconfigupdate"`
	NeedKeyUpdate    bool   `json:"needkeyupdate" bson:"needkeyupdate"`
	NeedDelete       bool   `json:"needdelete" bson:"needdelete"`
	NodeMessage      string `json:"nodemessage" bson:"nodemessage"`
	IsPending        bool   `json:"ispending" bson:"ispending"`
}

type PeersResponse struct {
	PublicKey           string `json:"publickey" bson:"publickey"`
	Endpoint            string `json:"endpoint" bson:"endpoint"`
	Address             string `json:"address" bson:"address"`
	Address6            string `json:"address6" bson:"address6"`
	LocalAddress        string `json:"localaddress" bson:"localaddress"`
	IsEgressGateway     string `json:"isegressgateway" bson:"isegressgateway"`
	EgressGatewayRanges string `json:"egressgatewayrange" bson:"egressgatewayrange"`
	ListenPort          int32  `json:"listenport" bson:"listenport"`
	KeepAlive           int32  `json:"persistentkeepalive" bson:"persistentkeepalive"`
}

type ExtPeersResponse struct {
	PublicKey    string `json:"publickey" bson:"publickey"`
	Endpoint     string `json:"endpoint" bson:"endpoint"`
	Address      string `json:"address" bson:"address"`
	Address6     string `json:"address6" bson:"address6"`
	LocalAddress string `json:"localaddress" bson:"localaddress"`
	ListenPort   int32  `json:"listenport" bson:"listenport"`
	KeepAlive    int32  `json:"persistentkeepalive" bson:"persistentkeepalive"`
}

type EgressGatewayRequest struct {
	NodeID      string   `json:"nodeid" bson:"nodeid"`
	NetID       string   `json:"netid" bson:"netid"`
	RangeString string   `json:"rangestring" bson:"rangestring"`
	Ranges      []string `json:"ranges" bson:"ranges"`
	Interface   string   `json:"interface" bson:"interface"`
	PostUp      string   `json:"postup" bson:"postup"`
	PostDown    string   `json:"postdown" bson:"postdown"`
}

type RelayRequest struct {
	NodeID      string   `json:"nodeid" bson:"nodeid"`
	NetID       string   `json:"netid" bson:"netid"`
	Addrs      []string `json:"addrs" bson:"addrs"`
}