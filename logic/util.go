// package for logicing client and server code
package logic

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/dnslogic"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/relay"
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
			functions.PrintUserLog(models.NODE_SERVER_NAME, "set new peers on network "+node.Network, 1)
		}
	} else {
		functions.PrintUserLog(models.NODE_SERVER_NAME, "could not set peers on network "+node.Network, 1)
		functions.PrintUserLog(models.NODE_SERVER_NAME, err.Error(), 1)
	}
}

// DeleteNode - deletes a node from database or moves into delete nodes table
func DeleteNode(key string, exterminate bool) error {
	var err error
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
			Log(err.Error(), 2)
		}
	}
	if err := database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		return err
	}
	if servercfg.IsDNSMode() {
		err = dnslogic.SetDNS()
	}
	return err
}

// CreateNode - creates a node in database
func CreateNode(node models.Node, networkName string) (models.Node, error) {

	//encrypt that password so we never see it
	hash, err := bcrypt.GenerateFromPassword([]byte(node.Password), 5)

	if err != nil {
		return node, err
	}
	//set password to encrypted password
	node.Password = string(hash)

	node.Network = networkName
	if node.Name == models.NODE_SERVER_NAME {
		node.IsServer = "yes"
	}
	if servercfg.IsDNSMode() && node.DNSOn == "" {
		node.DNSOn = "yes"
	}
	node.SetDefaults()
	node.Address, err = functions.UniqueAddress(networkName)
	if err != nil {
		return node, err
	}
	node.Address6, err = functions.UniqueAddress6(networkName)
	if err != nil {
		return node, err
	}
	//Create a JWT for the node
	tokenString, _ := functions.CreateJWT(node.MacAddress, networkName)
	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return node, err
	}
	err = node.Validate(false)
	if err != nil {
		return node, err
	}
	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return node, err
	}
	nodebytes, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	err = database.Insert(key, string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return node, err
	}
	if node.IsPending != "yes" {
		functions.DecrimentKey(node.Network, node.AccessKey)
	}
	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = dnslogic.SetDNS()
	}
	return node, err
}

// SetNetworkNodesLastModified - sets the network nodes last modified
func SetNetworkNodesLastModified(networkName string) error {

	timestamp := time.Now().Unix()

	network, err := functions.GetParentNetwork(networkName)
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

	key, err := functions.GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}
	data, err := database.FetchRecord(database.NODES_TABLE_NAME, key)
	if err != nil {
		if data == "" {
			data, err = database.FetchRecord(database.DELETED_NODES_TABLE_NAME, key)
			err = json.Unmarshal([]byte(data), &node)
		}
		return node, err
	}
	if err = json.Unmarshal([]byte(data), &node); err != nil {
		return node, err
	}
	node.SetDefaults()

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
		Log(err.Error(), 2)
		return nil, err
	}
	udppeers, errN := database.GetPeers(networkName)
	if errN != nil {
		Log(errN.Error(), 2)
	}
	for _, value := range collection {
		var node models.Node
		var peer models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			Log(err.Error(), 2)
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
				network, err := models.GetNetwork(networkName)
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
	var relayNode models.Node
	var err error
	if relayedNodeAddr == "" {
		peers, err = GetNodePeers(networkName, excludeRelayed)

	} else {
		relayNode, err = relay.GetNodeRelay(networkName, relayedNodeAddr)
		if relayNode.Address != "" {
			relayNode = setPeerInfo(relayNode)
			network, err := models.GetNetwork(networkName)
			if err == nil {
				relayNode.AllowedIPs = append(relayNode.AllowedIPs, network.AddressRange)
			} else {
				relayNode.AllowedIPs = append(relayNode.AllowedIPs, relayNode.RelayAddrs...)
			}
			nodepeers, err := GetNodePeers(networkName, false)
			if err == nil && relayNode.UDPHolePunch == "yes" {
				for _, nodepeer := range nodepeers {
					if nodepeer.Address == relayNode.Address {
						relayNode.Endpoint = nodepeer.Endpoint
						relayNode.ListenPort = nodepeer.ListenPort
					}
				}
			}

			peers = append(peers, relayNode)
		}
	}
	return peers, err
}

func setPeerInfo(node models.Node) models.Node {
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

func Log(message string, loglevel int) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	if int32(loglevel) <= servercfg.GetVerbose() && servercfg.GetVerbose() != 0 {
		log.Println("[netmaker] " + message)
	}
}

// == Private Methods ==

func setIPForwardingLinux() error {
	out, err := ncutils.RunCmd("sysctl net.ipv4.ip_forward", true)
	if err != nil {
		log.Println("WARNING: Error encountered setting ip forwarding. This can break functionality.")
		return err
	} else {
		s := strings.Fields(string(out))
		if s[2] != "1" {
			_, err = ncutils.RunCmd("sysctl -w net.ipv4.ip_forward=1", true)
			if err != nil {
				log.Println("WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				return err
			}
		}
	}
	return nil
}
