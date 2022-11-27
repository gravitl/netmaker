package node

var (
	networkInterface       string
	natEnabled             bool
	failover               bool
	networkName            string
	nodeDefinitionFilePath string
	endpoint               string
	listenPort             int
	address                string
	address6               string
	localAddress           string
	name                   string
	postUp                 string
	postDown               string
	allowedIPs             string
	keepAlive              int
	relayAddrs             string
	egressGatewayRanges    string
	localRange             string
	mtu                    int
	expirationDateTime     int
	defaultACL             bool
	dnsOn                  bool
	disconnect             bool
	networkHub             bool
)
