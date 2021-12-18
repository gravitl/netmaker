// package for logicing client and server code
package logic

import (
	"encoding/base64"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
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

// SetNetworkServerPeers - sets the network server peers of a given node
func SetNetworkServerPeers(node *models.Node) {
	if currentPeersList, err := GetSystemPeers(node); err == nil {
		if database.SetPeers(currentPeersList, node.Network) {
			logger.Log(1, "set new peers on network", node.Network)
		}
	} else {
		logger.Log(1, "could not set peers on network", node.Network, ":", err.Error())
	}
}

// DeleteNode - deletes a node from database or moves into delete nodes table
func DeleteNode(node *models.Node, exterminate bool) error {
	var err error
	node.SetID()
	var key = node.ID
	if !exterminate {
		args := strings.Split(key, "###")
		node, err := GetNode(args[0], args[1])
		if err != nil {
			return err
		}
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

// CreateNode - creates a node in database
func CreateNode(node *models.Node) error {

	//encrypt that password so we never see it
	hash, err := bcrypt.GenerateFromPassword([]byte(node.Password), 5)
	if err != nil {
		return err
	}
	//set password to encrypted password
	node.Password = string(hash)
	if node.Name == models.NODE_SERVER_NAME {
		node.IsServer = "yes"
	}
	if node.DNSOn == "" {
		if servercfg.IsDNSMode() {
			node.DNSOn = "yes"
		} else {
			node.DNSOn = "no"
		}
	}
	SetNodeDefaults(node)
	node.Address, err = UniqueAddress(node.Network)
	if err != nil {
		return err
	}
	node.Address6, err = UniqueAddress6(node.Network)
	if err != nil {
		return err
	}
	//Create a JWT for the node
	tokenString, _ := CreateJWT(node.MacAddress, node.Network)
	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return err
	}
	err = ValidateNode(node, false)
	if err != nil {
		return err
	}
	key, err := GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return err
	}
	nodebytes, err := json.Marshal(&node)
	if err != nil {
		return err
	}
	err = database.Insert(key, string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	if node.IsPending != "yes" {
		DecrimentKey(node.Network, node.AccessKey)
	}
	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}
	return err
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

// GetNode - fetches a node from database
func GetNode(macaddress string, network string) (models.Node, error) {
	var node models.Node

	key, err := GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}
	data, err := database.FetchRecord(database.NODES_TABLE_NAME, key)
	if err != nil {
		if data == "" {
			data, _ = database.FetchRecord(database.DELETED_NODES_TABLE_NAME, key)
			err = json.Unmarshal([]byte(data), &node)
		}
		return node, err
	}
	if err = json.Unmarshal([]byte(data), &node); err != nil {
		return node, err
	}
	SetNodeDefaults(&node)

	return node, err
}

// GetNodePeers - fetches peers for a given node
func GetNodePeers(networkName string, excludeRelayed bool) ([]models.Node, error) {
	var peers []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return peers, nil
		}
		logger.Log(2, err.Error())
		return nil, err
	}
	udppeers, errN := database.GetPeers(networkName)
	if errN != nil {
		logger.Log(2, errN.Error())
	}
	for _, value := range collection {
		var node = &models.Node{}
		var peer = models.Node{}
		err := json.Unmarshal([]byte(value), node)
		if err != nil {
			logger.Log(2, err.Error())
			continue
		}
		if node.IsEgressGateway == "yes" { // handle egress stuff
			peer.EgressGatewayRanges = node.EgressGatewayRanges
			peer.IsEgressGateway = node.IsEgressGateway
		}
		allow := node.IsRelayed != "yes" || !excludeRelayed

		if node.Network == networkName && node.IsPending != "yes" && allow {
			peer = setPeerInfo(node)
			if node.UDPHolePunch == "yes" && errN == nil && CheckEndpoint(udppeers[node.PublicKey]) {
				endpointstring := udppeers[node.PublicKey]
				endpointarr := strings.Split(endpointstring, ":")
				if len(endpointarr) == 2 {
					port, err := strconv.Atoi(endpointarr[1])
					if err == nil {
						peer.Endpoint = endpointarr[0]
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
			} else {
				peerNode.AllowedIPs = append(peerNode.AllowedIPs, peerNode.RelayAddrs...)
			}
			nodepeers, err := GetNodePeers(networkName, false)
			if err == nil && peerNode.UDPHolePunch == "yes" {
				for _, nodepeer := range nodepeers {
					if nodepeer.Address == peerNode.Address {
						peerNode.Endpoint = nodepeer.Endpoint
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
