package models

// ApiNode is a stripped down Node DTO that exposes only required fields to external systems
type ApiNode struct {
	ID                      string               `json:"id,omitempty" bson:"id,omitempty" yaml:"id,omitempty" validate:"required,min=5,id_unique"`
	HostID                  string               `json:"hostid,omitempty" bson:"id,omitempty" yaml:"hostid,omitempty" validate:"required,min=5,id_unique"`
	Address                 string               `json:"address" bson:"address" yaml:"address" validate:"omitempty,ipv4"`
	Address6                string               `json:"address6" bson:"address6" yaml:"address6" validate:"omitempty,ipv6"`
	LocalAddress            string               `json:"localaddress" bson:"localaddress" yaml:"localaddress" validate:"omitempty"`
	Interfaces              []Iface              `json:"interfaces" yaml:"interfaces"`
	Name                    string               `json:"name" bson:"name" yaml:"name" validate:"omitempty,max=62,in_charset"`
	NetworkSettings         Network              `json:"networksettings" bson:"networksettings" yaml:"networksettings" validate:"-"`
	ListenPort              int                  `json:"listenport" bson:"listenport" yaml:"listenport" validate:"omitempty,numeric,min=1024,max=65535"`
	LocalListenPort         int32                `json:"locallistenport" bson:"locallistenport" yaml:"locallistenport" validate:"numeric,min=0,max=65535"`
	ProxyListenPort         int32                `json:"proxy_listen_port" bson:"proxy_listen_port" yaml:"proxy_listen_port" validate:"numeric,min=0,max=65535"`
	PublicKey               string               `json:"publickey" bson:"publickey" yaml:"publickey" validate:"required,base64"`
	Endpoint                string               `json:"endpoint" bson:"endpoint" yaml:"endpoint" validate:"required,ip"`
	PostUp                  string               `json:"postup" bson:"postup" yaml:"postup"`
	PostDown                string               `json:"postdown" bson:"postdown" yaml:"postdown"`
	AllowedIPs              []string             `json:"allowedips" bson:"allowedips" yaml:"allowedips"`
	PersistentKeepalive     int32                `json:"persistentkeepalive" bson:"persistentkeepalive" yaml:"persistentkeepalive" validate:"omitempty,numeric,max=1000"`
	IsHub                   string               `json:"ishub" bson:"ishub" yaml:"ishub" validate:"checkyesorno"`
	LastModified            int64                `json:"lastmodified" bson:"lastmodified" yaml:"lastmodified"`
	ExpirationDateTime      int64                `json:"expdatetime" bson:"expdatetime" yaml:"expdatetime"`
	LastCheckIn             int64                `json:"lastcheckin" bson:"lastcheckin" yaml:"lastcheckin"`
	MacAddress              string               `json:"macaddress" bson:"macaddress" yaml:"macaddress"`
	Network                 string               `json:"network" bson:"network" yaml:"network" validate:"network_exists"`
	IsRelayed               bool                 `json:"isrelayed" bson:"isrelayed" yaml:"isrelayed"`
	IsPending               bool                 `json:"ispending" bson:"ispending" yaml:"ispending"`
	IsRelay                 bool                 `json:"isrelay" bson:"isrelay" yaml:"isrelay" validate:"checkyesorno"`
	IsDocker                bool                 `json:"isdocker" bson:"isdocker" yaml:"isdocker" validate:"checkyesorno"`
	IsK8S                   bool                 `json:"isk8s" bson:"isk8s" yaml:"isk8s" validate:"checkyesorno"`
	IsEgressGateway         bool                 `json:"isegressgateway" bson:"isegressgateway" yaml:"isegressgateway" validate:"checkyesorno"`
	IsIngressGateway        bool                 `json:"isingressgateway" bson:"isingressgateway" yaml:"isingressgateway" validate:"checkyesorno"`
	EgressGatewayRanges     []string             `json:"egressgatewayranges" bson:"egressgatewayranges" yaml:"egressgatewayranges"`
	EgressGatewayNatEnabled string               `json:"egressgatewaynatenabled" bson:"egressgatewaynatenabled" yaml:"egressgatewaynatenabled"`
	EgressGatewayRequest    EgressGatewayRequest `json:"egressgatewayrequest" bson:"egressgatewayrequest" yaml:"egressgatewayrequest"`
	RelayAddrs              []string             `json:"relayaddrs" bson:"relayaddrs" yaml:"relayaddrs"`
	FailoverNode            string               `json:"failovernode" bson:"failovernode" yaml:"failovernode"`
	IsStatic                string               `json:"isstatic" bson:"isstatic" yaml:"isstatic" validate:"checkyesorno"`
	DNSOn                   bool                 `json:"dnson" bson:"dnson" yaml:"dnson" validate:"checkyesorno"`
	IsLocal                 bool                 `json:"islocal" bson:"islocal" yaml:"islocal" validate:"checkyesorno"`
	LocalRange              string               `json:"localrange" bson:"localrange" yaml:"localrange"`
	IPForwarding            bool                 `json:"ipforwarding" bson:"ipforwarding" yaml:"ipforwarding" validate:"checkyesorno"`
	OS                      string               `json:"os" bson:"os" yaml:"os"`
	MTU                     int32                `json:"mtu" bson:"mtu" yaml:"mtu"`
	Version                 string               `json:"version" bson:"version" yaml:"version"`
	Server                  string               `json:"server" bson:"server" yaml:"server"`
	TrafficKeys             TrafficKeys          `json:"traffickeys" bson:"traffickeys" yaml:"traffickeys"`
	FirewallInUse           string               `json:"firewallinuse" bson:"firewallinuse" yaml:"firewallinuse"`
	InternetGateway         string               `json:"internetgateway" bson:"internetgateway" yaml:"internetgateway"`
	Connected               bool                 `json:"connected" bson:"connected" yaml:"connected" validate:"checkyesorno"`
	PendingDelete           bool                 `json:"pendingdelete" bson:"pendingdelete" yaml:"pendingdelete"`
	Proxy                   bool                 `json:"proxy" bson:"proxy" yaml:"proxy"`

	// == PRO ==
	DefaultACL string `json:"defaultacl,omitempty" bson:"defaultacl,omitempty" yaml:"defaultacl,omitempty" validate:"checkyesornoorunset"`
	Failover   string `json:"failover" bson:"failover" yaml:"failover" validate:"checkyesorno"`
}
