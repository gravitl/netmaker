package models

// APIHost - the host struct for API usage
type APIHost struct {
	ID              string   `json:"id"`
	Verbosity       int      `json:"verbosity"`
	FirewallInUse   string   `json:"firewallinuse"`
	Version         string   `json:"version"`
	IPForwarding    bool     `json:"ipforwarding"`
	DaemonInstalled bool     `json:"daemoninstalled"`
	HostPass        string   `json:"hostpass"`
	Name            string   `json:"name"`
	OS              string   `json:"os"`
	Interface       string   `json:"interface"`
	Debug           bool     `json:"debug"`
	ListenPort      int      `json:"listenport"`
	LocalAddress    string   `json:"localaddress"`
	LocalRange      string   `json:"localrange"`
	LocalListenPort int      `json:"locallistenport"`
	ProxyListenPort int      `json:"proxy_listen_port"`
	MTU             int      `json:"mtu" yaml:"mtu"`
	Interfaces      []Iface  `json:"interfaces" yaml:"interfaces"`
	PublicKey       string   `json:"publickey"`
	MacAddress      string   `json:"macaddress"`
	InternetGateway string   `json:"internetgateway"`
	Nodes           []string `json:"nodes"`
}
