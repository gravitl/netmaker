package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// CreateEgressGateway - creates an egress gateway
func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := GetNodeByID(gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	if node.OS != "linux" && node.OS != "freebsd" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for egress gateways")
	}
	if node.OS == "linux" && node.FirewallInUse == models.FIREWALL_NONE {
		return models.Node{}, errors.New("firewall is not supported for egress gateways")
	}
	if gateway.NatEnabled == "" {
		gateway.NatEnabled = "yes"
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	node.IsEgressGateway = "yes"
	node.EgressGatewayRanges = gateway.Ranges
	node.EgressGatewayNatEnabled = gateway.NatEnabled
	node.EgressGatewayRequest = gateway // store entire request for use when preserving the egress gateway
	postUpCmd := ""
	postDownCmd := ""
	ipv4, ipv6 := getNetworkProtocols(gateway.Ranges)
	logger.Log(3, "creating egress gateway firewall in use is '", node.FirewallInUse, "'")
	if node.OS == "linux" {
		switch node.FirewallInUse {
		case models.FIREWALL_NFTABLES:
			// nftables only supported on Linux
			// assumes chains eg FORWARD and postrouting already exist
			logger.Log(3, "creating egress gateway nftables is present")
			// down commands don't remove as removal of the rules leaves an empty chain while
			// removing the chain with rules in it would remove all rules in that section (not safe
			// if there are remaining rules on the host that need to stay).  In practice the chain is removed
			// when non-empty even though the removal of a non-empty chain should not be possible per nftables wiki.
			postUpCmd, postDownCmd = firewallNFTCommandsCreateEgress(node.Interface, gateway.Interface, gateway.Ranges, node.EgressGatewayNatEnabled, ipv4, ipv6)

		default: // iptables assumed
			logger.Log(3, "creating egress gateway nftables is not present")
			postUpCmd, postDownCmd = firewallIPTablesCommandsCreateEgress(node.Interface, gateway.Interface, node.EgressGatewayNatEnabled, ipv4, ipv6)
		}
	}
	if node.OS == "freebsd" {
		// spacing around ; is important for later parsing of postup/postdown in wireguard/common.go
		postUpCmd = "kldload ipfw ipfw_nat ; "
		postUpCmd += "ipfw disable one_pass ; "
		postUpCmd += "ipfw nat 1 config if " + gateway.Interface + " same_ports unreg_only reset ; "
		postUpCmd += "ipfw add 64000 reass all from any to any in ; "
		postUpCmd += "ipfw add 64000 nat 1 ip from any to any in via " + gateway.Interface + " ; "
		postUpCmd += "ipfw add 64000 check-state ; "
		postUpCmd += "ipfw add 64000 nat 1 ip from any to any out via " + gateway.Interface + " ; "
		postUpCmd += "ipfw add 65534 allow ip from any to any ; "
		postDownCmd = "ipfw delete 64000 ; "
		postDownCmd += "ipfw delete 65534 ; "
		postDownCmd += "kldunload ipfw_nat ipfw"

	}
	if gateway.PostUp != "" {
		postUpCmd = gateway.PostUp
	}
	if gateway.PostDown != "" {
		postDownCmd = gateway.PostDown
	}
	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			postUpCmd = node.PostUp + " ; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			postDownCmd = node.PostDown + " ; " + postDownCmd
		}
	}

	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.SetLastModified()
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	if err = database.Insert(node.ID, string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// ValidateEgressGateway - validates the egress gateway model
func ValidateEgressGateway(gateway models.EgressGatewayRequest) error {
	var err error

	empty := len(gateway.Ranges) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	empty = gateway.Interface == ""
	if empty {
		err = errors.New("interface cannot be empty")
	}
	return err
}

// DeleteEgressGateway - deletes egress from node
func DeleteEgressGateway(network, nodeid string) (models.Node, error) {

	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}

	node.IsEgressGateway = "no"
	node.EgressGatewayRanges = []string{}
	node.EgressGatewayRequest = models.EgressGatewayRequest{} // remove preserved request as the egress gateway is gone
	// needed in case we don't preserve a gateway (i.e., no ingress to preserve)
	node.PostUp = ""
	node.PostDown = ""
	cidrs := []string{}
	cidrs = append(cidrs, node.IngressGatewayRange)
	cidrs = append(cidrs, node.IngressGatewayRange6)
	ipv4, ipv6 := getNetworkProtocols(cidrs)
	logger.Log(3, "deleting egress gateway firewall in use is '", node.FirewallInUse, "'")
	if node.IsIngressGateway == "yes" { // check if node is still an ingress gateway before completely deleting postdown/up rules
		// still have an ingress gateway so preserve it
		if node.OS == "linux" {
			switch node.FirewallInUse {
			case models.FIREWALL_NFTABLES:
				// nftables only supported on Linux
				// assumes chains eg FORWARD and postrouting already exist
				logger.Log(3, "deleting egress gateway nftables is present")
				node.PostUp, node.PostDown = firewallNFTCommandsCreateIngress(node.Interface)
			default:
				logger.Log(3, "deleting egress gateway nftables is not present")
				node.PostUp, node.PostDown = firewallIPTablesCommandsCreateIngress(node.Interface, ipv4, ipv6)
			}
		}
		// no need to preserve ingress gateway on FreeBSD as ingress is not supported on that OS
	}
	node.SetLastModified()

	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	if err = database.Insert(node.ID, string(data), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// CreateIngressGateway - creates an ingress gateway
func CreateIngressGateway(netid string, nodeid string) (models.Node, error) {

	var postUpCmd, postDownCmd string
	node, err := GetNodeByID(nodeid)
	if node.OS != "linux" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for ingress gateways")
	}
	if node.OS == "linux" && node.FirewallInUse == models.FIREWALL_NONE {
		return models.Node{}, errors.New("firewall is not supported for ingress gateways")
	}

	if err != nil {
		return models.Node{}, err
	}

	network, err := GetParentNetwork(netid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsIngressGateway = "yes"
	cidrs := []string{}
	cidrs = append(cidrs, network.AddressRange)
	cidrs = append(cidrs, network.AddressRange6)
	node.IngressGatewayRange = network.AddressRange
	node.IngressGatewayRange6 = network.AddressRange6
	ipv4, ipv6 := getNetworkProtocols(cidrs)
	logger.Log(3, "creating ingress gateway firewall in use is '", node.FirewallInUse, "'")
	switch node.FirewallInUse {
	case models.FIREWALL_NFTABLES:
		// nftables only supported on Linux
		// assumes chains eg FORWARD and postrouting already exist
		logger.Log(3, "creating ingress gateway nftables is present")
		postUpCmd, postDownCmd = firewallNFTCommandsCreateIngress(node.Interface)
	default:
		logger.Log(3, "creating ingress gateway using nftables is not present")
		postUpCmd, postDownCmd = firewallIPTablesCommandsCreateIngress(node.Interface, ipv4, ipv6)
	}

	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			postUpCmd = node.PostUp + " ; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			postDownCmd = node.PostDown + " ; " + postDownCmd
		}
	}
	node.SetLastModified()
	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.UDPHolePunch = "no"

	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(netid)
	return node, err
}

// DeleteIngressGateway - deletes an ingress gateway
func DeleteIngressGateway(networkName string, nodeid string) (models.Node, error) {

	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}
	network, err := GetParentNetwork(networkName)
	if err != nil {
		return models.Node{}, err
	}
	// delete ext clients belonging to ingress gateway
	if err = DeleteGatewayExtClients(node.ID, networkName); err != nil {
		return models.Node{}, err
	}
	logger.Log(3, "deleting ingress gateway")

	node.UDPHolePunch = network.DefaultUDPHolePunch
	node.LastModified = time.Now().Unix()
	node.IsIngressGateway = "no"
	node.IngressGatewayRange = ""

	// default to removing postup and postdown
	node.PostUp = ""
	node.PostDown = ""

	logger.Log(3, "deleting ingress gateway firewall in use is '", node.FirewallInUse, "' and isEgressGateway is", node.IsEgressGateway)
	if node.EgressGatewayRequest.NodeID != "" {
		_, err := CreateEgressGateway(node.EgressGatewayRequest)
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to create egress gateway on node [%s] on network [%s]: %v",
				node.EgressGatewayRequest.NodeID, node.EgressGatewayRequest.NetID, err))
		}
	}

	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(networkName)
	return node, err
}

// DeleteGatewayExtClients - deletes ext clients based on gateway (mac) of ingress node and network
func DeleteGatewayExtClients(gatewayID string, networkName string) error {
	currentExtClients, err := GetNetworkExtClients(networkName)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	for _, extClient := range currentExtClients {
		if extClient.IngressGatewayID == gatewayID {
			if err = DeleteExtClient(networkName, extClient.ClientID); err != nil {
				logger.Log(1, "failed to remove ext client", extClient.ClientID)
				continue
			}
		}
	}
	return nil
}

// firewallNFTCommandsCreateIngress - used to centralize firewall command maintenance for creating an ingress gateway using the nftables firewall.
func firewallNFTCommandsCreateIngress(networkInterface string) (string, string) {
	// spacing around ; is important for later parsing of postup/postdown in wireguard/common.go
	postUp := "nft add table ip filter ; "
	postUp += "nft add chain ip filter FORWARD ; "
	postUp += "nft add rule ip filter FORWARD iifname " + networkInterface + " counter accept ; "
	postUp += "nft add rule ip filter FORWARD oifname " + networkInterface + " counter accept ; "
	postUp += "nft add table nat ; "
	postUp += "nft add chain nat postrouting ; "
	postUp += "nft add rule ip nat postrouting oifname " + networkInterface + " counter masquerade"

	// doesn't remove potentially empty tables or chains
	postDown := "nft flush table filter ; "
	postDown += "nft flush table nat ; "

	return postUp, postDown
}

// firewallNFTCommandsCreateEgress - used to centralize firewall command maintenance for creating an egress gateway using the nftables firewall.
func firewallNFTCommandsCreateEgress(networkInterface string, gatewayInterface string, gatewayranges []string, egressNatEnabled string, ipv4, ipv6 bool) (string, string) {
	// spacing around ; is important for later parsing of postup/postdown in wireguard/common.go
	postUp := ""
	postDown := ""
	if ipv4 {
		postUp += "nft add table ip filter ; "
		postUp += "nft add chain ip filter forward ; "
		postUp += "nft add rule filter forward ct state related,established accept ; "
		postUp += "nft add rule ip filter forward iifname " + networkInterface + " accept ; "
		postUp += "nft add rule ip filter forward oifname " + networkInterface + " accept ; "
		postUp += "nft add table nat ; "
		postUp += "nft 'add chain ip nat prerouting { type nat hook prerouting priority 0 ;}' ; "
		postUp += "nft 'add chain ip nat postrouting { type nat hook postrouting priority 0 ;}' ; "
		for _, networkCIDR := range gatewayranges {
			postUp += "nft add rule nat postrouting iifname " + networkInterface + " oifname " + gatewayInterface + " ip saddr " + networkCIDR + " masquerade ; "
		}

		postDown += "nft flush table filter ; "

		if egressNatEnabled == "yes" {
			postUp += "nft add table nat ; "
			postUp += "nft add chain nat postrouting ; "
			postUp += "nft add rule ip nat postrouting oifname " + gatewayInterface + " counter masquerade ; "

			postDown += "nft flush table nat ; "
		}
	}
	if ipv6 {
		postUp += "nft add table ip6 filter ; "
		postUp += "nft add chain ip6 filter forward ; "
		postUp += "nft add rule ip6 filter forward ct state related,established accept ; "
		postUp += "nft add rule ip6 filter forward iifname " + networkInterface + " accept ; "
		postUp += "nft add rule ip6 filter forward oifname " + networkInterface + " accept ; "

		postDown += "nft flush table ip6 filter ; "

		if egressNatEnabled == "yes" {
			postUp += "nft add table ip6 nat ; "
			postUp += "nft 'add chain ip6 nat prerouting { type nat hook prerouting priority 0 ;}' ; "
			postUp += "nft 'add chain ip6 nat postrouting { type nat hook postrouting priority 0 ;}' ; "
			postUp += "nft add rule ip6 nat postrouting oifname " + gatewayInterface + " masquerade ; "

			postDown += "nft flush table ip6 nat ; "
		}
	}

	return postUp, postDown
}

// firewallIPTablesCommandsCreateIngress - used to centralize firewall command maintenance for creating an ingress gateway using the iptables firewall.
func firewallIPTablesCommandsCreateIngress(networkInterface string, ipv4, ipv6 bool) (string, string) {
	postUp := ""
	postDown := ""
	if ipv4 {
		// spacing around ; is important for later parsing of postup/postdown in wireguard/common.go
		postUp += "iptables -A FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postUp += "iptables -A FORWARD -o " + networkInterface + " -j ACCEPT ; "
		postUp += "iptables -t nat -A POSTROUTING -o " + networkInterface + " -j MASQUERADE ; "

		// doesn't remove potentially empty tables or chains
		postDown += "iptables -D FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postDown += "iptables -D FORWARD -o " + networkInterface + " -j ACCEPT ; "
		postDown += "iptables -t nat -D POSTROUTING -o " + networkInterface + " -j MASQUERADE ; "
	}
	if ipv6 {
		// spacing around ; is important for later parsing of postup/postdown in wireguard/common.go
		postUp += "ip6tables -A FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postUp += "ip6tables -A FORWARD -o " + networkInterface + " -j ACCEPT ; "
		postUp += "ip6tables -t nat -A POSTROUTING -o " + networkInterface + " -j MASQUERADE"

		// doesn't remove potentially empty tables or chains
		postDown += "ip6tables -D FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postDown += "ip6tables -D FORWARD -o " + networkInterface + " -j ACCEPT ; "
		postDown += "ip6tables -t nat -D POSTROUTING -o " + networkInterface + " -j MASQUERADE"
	}
	return postUp, postDown
}

// firewallIPTablesCommandsCreateEgress - used to centralize firewall command maintenance for creating an egress gateway using the iptables firewall.
func firewallIPTablesCommandsCreateEgress(networkInterface string, gatewayInterface string, egressNatEnabled string, ipv4, ipv6 bool) (string, string) {
	// spacing around ; is important for later parsing of postup/postdown in wireguard/common.go
	postUp := ""
	postDown := ""
	if ipv4 {
		postUp += "iptables -A FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postUp += "iptables -A FORWARD -o " + networkInterface + " -j ACCEPT"
		postDown += "iptables -D FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postDown += "iptables -D FORWARD -o " + networkInterface + " -j ACCEPT ; "

		if egressNatEnabled == "yes" {
			postUp += " ; iptables -t nat -A POSTROUTING -o " + gatewayInterface + " -j MASQUERADE ; "
			postDown += " ; iptables -t nat -D POSTROUTING -o " + gatewayInterface + " -j MASQUERADE ; "
		}
	}
	if ipv6 {
		postUp += "ip6tables -A FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postUp += "ip6tables -A FORWARD -o " + networkInterface + " -j ACCEPT ; "
		postDown += "ip6tables -D FORWARD -i " + networkInterface + " -j ACCEPT ; "
		postDown += "ip6tables -D FORWARD -o " + networkInterface + " -j ACCEPT ; "

		if egressNatEnabled == "yes" {
			postUp += " ; ip6tables -t nat -A POSTROUTING -o " + gatewayInterface + " -j MASQUERADE"
			postDown += " ; ip6tables -t nat -D POSTROUTING -o " + gatewayInterface + " -j MASQUERADE"
		}
	}
	return postUp, postDown

}
