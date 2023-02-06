package node

var (
	natEnabled             bool
	failover               bool
	networkName            string
	nodeDefinitionFilePath string
	address                string
	address6               string
	localAddress           string
	name                   string
	postUp                 string
	postDown               string
	keepAlive              int
	relayAddrs             string
	egressGatewayRanges    string
	expirationDateTime     int
	defaultACL             bool
	dnsOn                  bool
	disconnect             bool
)
