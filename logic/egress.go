package logic

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"slices"
	"sort"

	"github.com/gravitl/netmaker/models"
)

func AutoConfigureEgress(h *models.Host, node *models.Node) {
	currRangesWithMetric := GetEgressRangesWithMetric(models.NetworkID(node.Network))
	ranges := []string{}
	rangesWithMetric := []models.EgressRangeMetric{}
	for _, iface := range h.Interfaces {
		if !iface.Address.IP.IsPrivate() || iface.Name == h.DefaultInterface {
			continue
		}
		addr, err := NormalizeCIDR(iface.Address.String())
		if err == nil {
			ranges = append(ranges, addr)
		}
		rangeWithMetric := models.EgressRangeMetric{
			Network:           addr,
			VirtualNATNetwork: iface.VirtualNATAddr.String(),
			RouteMetric:       256,
		}
		if currRangeMetric, ok := currRangesWithMetric[addr]; ok {
			lastMetricValue := currRangeMetric[len(currRangeMetric)-1]
			rangeWithMetric.RouteMetric = lastMetricValue.RouteMetric + 10
		}
		rangesWithMetric = append(rangesWithMetric, rangeWithMetric)
	}
	CreateEgressGateway(models.EgressGatewayRequest{
		NodeID:           node.ID.String(),
		NetID:            node.Network,
		NatEnabled:       "yes",
		Ranges:           ranges,
		RangesWithMetric: rangesWithMetric,
	})
}

func MapExternalServicesToHostNodes(h *models.Host) {
	ranges := []string{}
	rangesWithMetric := []models.EgressRangeMetric{}
	for _, nodeID := range h.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		for i, egressServiceIPs := range h.EgressServices {
			for j, egressIPNat := range egressServiceIPs {
				currRangesWithMetric := GetEgressRangesWithMetric(models.NetworkID(node.Network))

				addr := egressIPNat.EgressIP.String()
				ranges = append(ranges, addr)
				for _, iface := range h.Interfaces {
					if !iface.Address.IP.IsPrivate() || iface.Name == h.DefaultInterface {
						continue
					}
					ifaceAddr, err := NormalizeCIDRV1(iface.Address.String())
					if err != nil {
						continue
					}
					if !ifaceAddr.Contains(egressIPNat.EgressIP) {
						continue
					}
					egressNATIP, err := netmapTranslate(egressIPNat.EgressIP, ifaceAddr.String(), iface.VirtualNATAddr.String())
					if err != nil {
						continue
					}
					egressIPNat.NATIP = egressNATIP
					egressServiceIPs[j] = egressIPNat
					rangeWithMetric := models.EgressRangeMetric{
						Network:           addr,
						VirtualNATNetwork: egressNATIP.String(),
						RouteMetric:       256,
					}
					if currRangeMetric, ok := currRangesWithMetric[addr]; ok {
						lastMetricValue := currRangeMetric[len(currRangeMetric)-1]
						rangeWithMetric.RouteMetric = lastMetricValue.RouteMetric + 10
					}
					rangesWithMetric = append(rangesWithMetric, rangeWithMetric)
				}
				h.EgressServices[i] = egressServiceIPs

			}

		}
		if !node.IsEgressGateway {
			CreateEgressGateway(models.EgressGatewayRequest{
				NodeID:           node.ID.String(),
				NetID:            node.Network,
				NatEnabled:       "yes",
				Ranges:           ranges,
				RangesWithMetric: rangesWithMetric,
			})
		} else {
			node.EgressGatewayRequest.Ranges = append(node.EgressGatewayRequest.Ranges, ranges...)
			node.EgressGatewayRequest.RangesWithMetric = append(node.EgressGatewayRequest.RangesWithMetric, rangesWithMetric...)
			node.EgressGatewayRanges = append(node.EgressGatewayRanges, ranges...)
			UpsertNode(&node)
		}

	}
	UpsertHost(h)
}

// netmapTranslate translates an IP address from one subnet to another
func netmapTranslate(ip net.IP, srcCIDR, dstCIDR string) (net.IP, error) {
	_, srcNet, err := net.ParseCIDR(srcCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid source CIDR: %w", err)
	}

	_, dstNet, err := net.ParseCIDR(dstCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid destination CIDR: %w", err)
	}

	// Ensure IP belongs to the source subnet
	if !srcNet.Contains(ip) {
		return nil, fmt.Errorf("IP %s not in source CIDR %s", ip, srcCIDR)
	}

	// Convert IPs to big.Int
	ipInt := ipToBigInt(ip)
	srcBase := ipToBigInt(srcNet.IP)
	dstBase := ipToBigInt(dstNet.IP)

	// Compute offset and apply it to the destination base
	offset := new(big.Int).Sub(ipInt, srcBase)
	translatedIP := new(big.Int).Add(dstBase, offset)

	return bigIntToIP(translatedIP, ip), nil
}

// ipToBigInt converts an IP address to a big integer
func ipToBigInt(ip net.IP) *big.Int {
	return new(big.Int).SetBytes(ip.To16()) // Always use IPv6 representation
}

// bigIntToIP correctly converts a big integer back to an IP address
func bigIntToIP(bi *big.Int, originalIP net.IP) net.IP {
	ipBytes := bi.Bytes()

	// Ensure correct length (IPv4 = 4 bytes, IPv6 = 16 bytes)
	if len(originalIP) == net.IPv4len && len(ipBytes) > 4 {
		ipBytes = ipBytes[len(ipBytes)-4:] // Extract last 4 bytes for IPv4
	} else if len(ipBytes) < len(originalIP) {
		padding := make([]byte, len(originalIP)-len(ipBytes))
		ipBytes = append(padding, ipBytes...)
	}

	return net.IP(ipBytes)
}

// isConflicting checks if a given CIDR conflicts with existing subnets
func isConflicting(cidr *net.IPNet, existing []net.IPNet) bool {
	for _, net := range existing {
		if cidr.Contains(net.IP) || net.Contains(cidr.IP) {
			return true
		}
	}
	return false
}

// AssignVirtualNATs assigns a unique virtual NAT subnet for each interface CIDR
func AssignVirtualNATs(h *models.Host) error {
	existingNets := []net.IPNet{}
	for _, nodeID := range h.Nodes {
		node, err := GetNodeByID(nodeID)
		if err == nil {
			existingNets = append(existingNets, node.NetworkRange, node.Address6)
		}

	}

	for _, iface := range h.Interfaces {
		existingNets = append(existingNets, iface.Address)
	}
	ipv4Index := 1
	ipv6Index := 1
	for i, iface := range h.Interfaces {
		if !iface.Address.IP.IsPrivate() || iface.Name == h.DefaultInterface {
			continue
		}
		// Preserve the original subnet mask
		ones, _ := iface.Address.Mask.Size()

		var newSubnet string
		if iface.Address.IP.To4() != nil { // IPv4 case
			newSubnet = fmt.Sprintf("10.64.%d.0/%d", ipv4Index, ones)
			ipv4Index++
		} else { // IPv6 case
			newSubnet = fmt.Sprintf("fd00:64:%d::/%d", ipv6Index, ones)
			ipv6Index++
		}

		_, candidateNet, _ := net.ParseCIDR(newSubnet)

		// Ensure no conflicts
		if !isConflicting(candidateNet, existingNets) {
			_, newSubnetCidr, _ := net.ParseCIDR(newSubnet)
			h.Interfaces[i].VirtualNATAddr = *newSubnetCidr
		} else {
			return fmt.Errorf("could not find non-conflicting subnet for %s", iface.Address.String())
		}
	}
	return nil
}

func GetEgressRanges(netID models.NetworkID) (map[string][]string, map[string]struct{}, error) {

	resultMap := make(map[string]struct{})
	nodeEgressMap := make(map[string][]string)
	networkNodes, err := GetNetworkNodes(netID.String())
	if err != nil {
		return nil, nil, err
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != netID.String() {
			continue
		}
		if currentNode.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressGatewayRanges) > 0 {
				nodeEgressMap[currentNode.ID.String()] = currentNode.EgressGatewayRanges
				for _, egressRangeI := range currentNode.EgressGatewayRanges {
					resultMap[egressRangeI] = struct{}{}
				}
			}
		}
	}
	extclients, _ := GetNetworkExtClients(netID.String())
	for _, extclient := range extclients {
		if len(extclient.ExtraAllowedIPs) > 0 {
			nodeEgressMap[extclient.ClientID] = extclient.ExtraAllowedIPs
			for _, extraAllowedIP := range extclient.ExtraAllowedIPs {
				resultMap[extraAllowedIP] = struct{}{}
			}
		}
	}
	return nodeEgressMap, resultMap, nil
}
func sortRouteMetricByAscending(items []models.EgressRangeMetric) []models.EgressRangeMetric {
	sort.Slice(items, func(i, j int) bool {
		return items[i].RouteMetric < items[j].RouteMetric
	})
	return items
}

func GetEgressRangesWithMetric(netID models.NetworkID) map[string][]models.EgressRangeMetric {

	egressMap := make(map[string][]models.EgressRangeMetric)
	networkNodes, err := GetNetworkNodes(netID.String())
	if err != nil {
		return nil
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != netID.String() {
			continue
		}
		if currentNode.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressGatewayRequest.RangesWithMetric) > 0 {
				for _, egressRangeI := range currentNode.EgressGatewayRequest.RangesWithMetric {
					if value, ok := egressMap[egressRangeI.Network]; ok {
						value = append(value, egressRangeI)
						egressMap[egressRangeI.Network] = value
					} else {
						egressMap[egressRangeI.Network] = []models.EgressRangeMetric{
							egressRangeI,
						}
					}
				}
			}
		}
	}
	for key, value := range egressMap {
		egressMap[key] = sortRouteMetricByAscending(value)
	}
	return egressMap
}

// GetAllEgresses - gets all the nodes that are egresses
func GetAllEgresses() ([]models.Node, error) {
	nodes, err := GetAllNodes()
	if err != nil {
		return nil, err
	}
	egresses := make([]models.Node, 0)
	for _, node := range nodes {
		if node.IsEgressGateway {
			egresses = append(egresses, node)
		}
	}
	return egresses, nil
}

// CreateEgressGateway - creates an egress gateway
func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := GetNodeByID(gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return models.Node{}, err
	}
	if host.OS != "linux" { // support for other OS to be added
		return models.Node{}, errors.New(host.OS + " is unsupported for egress gateways")
	}
	if host.FirewallInUse == models.FIREWALL_NONE {
		return models.Node{}, errors.New("please install iptables or nftables on the device")
	}
	if len(gateway.RangesWithMetric) == 0 && len(gateway.Ranges) > 0 {
		for _, rangeI := range gateway.Ranges {
			gateway.RangesWithMetric = append(gateway.RangesWithMetric, models.EgressRangeMetric{
				Network:     rangeI,
				RouteMetric: 256,
			})
		}
	}
	for i := len(gateway.Ranges) - 1; i >= 0; i-- {
		// check if internet gateway IPv4
		if gateway.Ranges[i] == "0.0.0.0/0" || gateway.Ranges[i] == "::/0" {
			// remove inet range
			gateway.Ranges = append(gateway.Ranges[:i], gateway.Ranges[i+1:]...)
			continue
		}
		normalized, err := NormalizeCIDR(gateway.Ranges[i])
		if err != nil {
			return models.Node{}, err
		}
		gateway.Ranges[i] = normalized

	}
	rangesWithMetric := []string{}
	for i := len(gateway.RangesWithMetric) - 1; i >= 0; i-- {
		if gateway.RangesWithMetric[i].Network == "0.0.0.0/0" || gateway.RangesWithMetric[i].Network == "::/0" {
			// remove inet range
			gateway.RangesWithMetric = append(gateway.RangesWithMetric[:i], gateway.RangesWithMetric[i+1:]...)
			continue
		}
		normalized, err := NormalizeCIDR(gateway.RangesWithMetric[i].Network)
		if err != nil {
			return models.Node{}, err
		}
		gateway.RangesWithMetric[i].Network = normalized
		rangesWithMetric = append(rangesWithMetric, gateway.RangesWithMetric[i].Network)
		if gateway.RangesWithMetric[i].RouteMetric <= 0 || gateway.RangesWithMetric[i].RouteMetric > 999 {
			gateway.RangesWithMetric[i].RouteMetric = 256
		}
	}
	sort.Strings(gateway.Ranges)
	sort.Strings(rangesWithMetric)
	if !slices.Equal(gateway.Ranges, rangesWithMetric) {
		return models.Node{}, errors.New("invalid ranges")
	}
	if gateway.NatEnabled == "" {
		gateway.NatEnabled = "yes"
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	if gateway.Ranges == nil {
		gateway.Ranges = make([]string, 0)
	}
	node.IsEgressGateway = true
	node.EgressGatewayRanges = gateway.Ranges
	node.EgressGatewayNatEnabled = models.ParseBool(gateway.NatEnabled)

	node.EgressGatewayRequest = gateway // store entire request for use when preserving the egress gateway
	node.SetLastModified()
	if err = UpsertNode(&node); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// ValidateEgressGateway - validates the egress gateway model
func ValidateEgressGateway(gateway models.EgressGatewayRequest) error {
	return nil
}

// DeleteEgressGateway - deletes egress from node
func DeleteEgressGateway(network, nodeid string) (models.Node, error) {
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsEgressGateway = false
	node.EgressGatewayRanges = []string{}
	node.EgressGatewayRequest = models.EgressGatewayRequest{} // remove preserved request as the egress gateway is gone
	node.SetLastModified()
	if err = UpsertNode(&node); err != nil {
		return models.Node{}, err
	}
	return node, nil
}
