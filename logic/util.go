// package for logicing client and server code
package logic

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// IsBase64 - checks if a string is in base64 format
// This is used to validate public keys (make sure they're base64 encoded like all public keys should be).
func IsBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// CheckEndpoint - checks if an endpoint is valid
func CheckEndpoint(endpoint string) bool {
	endpointarr := strings.Split(endpoint, ":")
	return len(endpointarr) == 2
}

// FileExists - checks if local file exists
func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// IsAddressInCIDR - util to see if an address is in a cidr or not
func IsAddressInCIDR(address, cidr string) bool {
	var _, currentCIDR, cidrErr = net.ParseCIDR(cidr)
	if cidrErr != nil {
		return false
	}
	var addrParts = strings.Split(address, ".")
	var addrPartLength = len(addrParts)
	if addrPartLength != 4 {
		return false
	} else {
		if addrParts[addrPartLength-1] == "0" ||
			addrParts[addrPartLength-1] == "255" {
			return false
		}
	}
	ip, _, err := net.ParseCIDR(fmt.Sprintf("%s/32", address))
	if err != nil {
		return false
	}
	return currentCIDR.Contains(ip)
}

// SetNetworkNodesLastModified - sets the network nodes last modified
func SetNetworkNodesLastModified(networkName string) error {

	timestamp := time.Now().Unix()

	network, err := GetParentNetwork(networkName)
	if err != nil {
		return err
	}
	network.NodesLastModified = timestamp
	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	err = database.Insert(networkName, string(data), database.NETWORKS_TABLE_NAME)
	if err != nil {
		return err
	}
	return nil
}

// GenerateCryptoString - generates random string of n length
func GenerateCryptoString(n int) (string, error) {
	const chars = "123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := range ret {
		num, err := crand.Int(crand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		ret[i] = chars[num.Int64()]
	}

	return string(ret), nil
}

// DeleteNodeByID - deletes a node from database or moves into delete nodes table
func DeleteNodeByID(node *models.Node, exterminate bool) error {
	var err error
	var key = node.ID
	if !exterminate {
		node.Action = models.NODE_DELETE
		nodedata, err := json.Marshal(&node)
		if err != nil {
			return err
		}
		err = database.Insert(key, string(nodedata), database.DELETED_NODES_TABLE_NAME)
		if err != nil {
			return err
		}
	} else {
		if err := database.DeleteRecord(database.DELETED_NODES_TABLE_NAME, key); err != nil {
			logger.Log(2, err.Error())
		}
	}
	if err = database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		return err
	}
	if servercfg.IsDNSMode() {
		SetDNS()
	}
	return removeLocalServer(node)
}

// GetNodePeers - fetches peers for a given node
func GetNodePeers(networkName string, excludeRelayed bool) ([]models.Node, error) {
	var peers []models.Node
	var networkNodes, egressNetworkNodes, err = getNetworkEgressAndNodes(networkName)
	if err != nil {
		return peers, nil
	}

	udppeers, errN := database.GetPeers(networkName)
	if errN != nil {
		logger.Log(2, errN.Error())
	}

	for _, node := range networkNodes {
		var peer = models.Node{}
		if node.IsEgressGateway == "yes" { // handle egress stuff
			peer.EgressGatewayRanges = node.EgressGatewayRanges
			peer.IsEgressGateway = node.IsEgressGateway
		}
		allow := node.IsRelayed != "yes" || !excludeRelayed

		if node.Network == networkName && node.IsPending != "yes" && allow {
			peer = setPeerInfo(&node)
			if node.UDPHolePunch == "yes" && errN == nil && CheckEndpoint(udppeers[node.PublicKey]) {
				endpointstring := udppeers[node.PublicKey]
				endpointarr := strings.Split(endpointstring, ":")
				if len(endpointarr) == 2 {
					port, err := strconv.Atoi(endpointarr[1])
					if err == nil {
						// peer.Endpoint = endpointarr[0]
						peer.ListenPort = int32(port)
					}
				}
			}
			if node.IsRelay == "yes" {
				network, err := GetNetwork(networkName)
				if err == nil {
					peer.AllowedIPs = append(peer.AllowedIPs, network.AddressRange)
				} else {
					peer.AllowedIPs = append(peer.AllowedIPs, node.RelayAddrs...)
				}
				for _, egressNode := range egressNetworkNodes {
					if egressNode.IsRelayed == "yes" && StringSliceContains(node.RelayAddrs, egressNode.Address) {
						peer.AllowedIPs = append(peer.AllowedIPs, egressNode.EgressGatewayRanges...)
					}
				}
			}
			peers = append(peers, peer)
		}
	}

	return peers, err
}

// GetPeersList - gets the peers of a given network
func GetPeersList(networkName string, excludeRelayed bool, relayedNodeAddr string) ([]models.Node, error) {
	var peers []models.Node
	var err error
	if relayedNodeAddr == "" {
		peers, err = GetNodePeers(networkName, excludeRelayed)
	} else {
		var relayNode models.Node
		relayNode, err = GetNodeRelay(networkName, relayedNodeAddr)
		if relayNode.Address != "" {
			var peerNode = setPeerInfo(&relayNode)
			network, err := GetNetwork(networkName)
			if err == nil {
				peerNode.AllowedIPs = append(peerNode.AllowedIPs, network.AddressRange)
				var _, egressNetworkNodes, err = getNetworkEgressAndNodes(networkName)
				if err == nil {
					for _, egress := range egressNetworkNodes {
						if egress.Address != relayedNodeAddr {
							peerNode.AllowedIPs = append(peerNode.AllowedIPs, egress.EgressGatewayRanges...)
						}
					}
				}
			} else {
				peerNode.AllowedIPs = append(peerNode.AllowedIPs, peerNode.RelayAddrs...)
			}
			nodepeers, err := GetNodePeers(networkName, false)
			if err == nil && peerNode.UDPHolePunch == "yes" {
				for _, nodepeer := range nodepeers {
					if nodepeer.Address == peerNode.Address {
						// peerNode.Endpoint = nodepeer.Endpoint
						peerNode.ListenPort = nodepeer.ListenPort
					}
				}
			}

			peers = append(peers, peerNode)
		}
	}
	return peers, err
}

// RandomString - returns a random string in a charset
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// == Private Methods ==

func getNetworkEgressAndNodes(networkName string) ([]models.Node, []models.Node, error) {
	var networkNodes, egressNetworkNodes []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return networkNodes, egressNetworkNodes, nil
		}
		logger.Log(2, err.Error())
		return nil, nil, err
	}

	for _, value := range collection {
		var node = models.Node{}
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			logger.Log(2, err.Error())
			continue
		}
		if node.Network == networkName {
			networkNodes = append(networkNodes, node)
			if node.IsEgressGateway == "yes" {
				egressNetworkNodes = append(egressNetworkNodes, node)
			}
		}
	}
	return networkNodes, egressNetworkNodes, nil
}

func setPeerInfo(node *models.Node) models.Node {
	var peer models.Node
	peer.RelayAddrs = node.RelayAddrs
	peer.IsRelay = node.IsRelay
	peer.IsServer = node.IsServer
	peer.IsRelayed = node.IsRelayed
	peer.PublicKey = node.PublicKey
	peer.Endpoint = node.Endpoint
	peer.Name = node.Name
	peer.LocalAddress = node.LocalAddress
	peer.ListenPort = node.ListenPort
	peer.AllowedIPs = node.AllowedIPs
	peer.UDPHolePunch = node.UDPHolePunch
	peer.Address = node.Address
	peer.Address6 = node.Address6
	peer.EgressGatewayRanges = node.EgressGatewayRanges
	peer.IsEgressGateway = node.IsEgressGateway
	peer.IngressGatewayRange = node.IngressGatewayRange
	peer.IsIngressGateway = node.IsIngressGateway
	peer.IsPending = node.IsPending
	return peer
}

func setIPForwardingLinux() error {
	out, err := ncutils.RunCmd("sysctl net.ipv4.ip_forward", true)
	if err != nil {
		logger.Log(0, "WARNING: Error encountered setting ip forwarding. This can break functionality.")
		return err
	} else {
		s := strings.Fields(string(out))
		if s[2] != "1" {
			_, err = ncutils.RunCmd("sysctl -w net.ipv4.ip_forward=1", true)
			if err != nil {
				logger.Log(0, "WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				return err
			}
		}
	}
	return nil
}

// StringSliceContains - sees if a string slice contains a string element
func StringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// == private ==

// sets the network server peers of a given node
func setNetworkServerPeers(serverNode *models.Node) {
	if currentPeersList, err := getSystemPeers(serverNode); err == nil {
		if database.SetPeers(currentPeersList, serverNode.Network) {
			logger.Log(1, "set new peers on network", serverNode.Network)
		}
	} else {
		logger.Log(1, "could not set peers on network", serverNode.Network, ":", err.Error())
	}
}

// ShouldPublishPeerPorts - Gets ports from iface, sets, and returns true if they are different
func ShouldPublishPeerPorts(serverNode *models.Node) bool {
	if currentPeersList, err := getSystemPeers(serverNode); err == nil {
		if database.SetPeers(currentPeersList, serverNode.Network) {
			logger.Log(1, "set new peers on network", serverNode.Network)
			return true
		}
	}
	return false
}
