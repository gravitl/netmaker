package logic

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// == Join, Checkin, and Leave for Server ==

// KUBERNETES_LISTEN_PORT - starting port for Kubernetes in order to use NodePort range
const KUBERNETES_LISTEN_PORT = 31821

// KUBERNETES_SERVER_MTU - ideal mtu for kubernetes deployments right now
const KUBERNETES_SERVER_MTU = 1024

// ServerJoin - responsible for joining a server to a network
func ServerJoin(networkSettings *models.Network) (models.Node, error) {
	var returnNode models.Node
	if networkSettings == nil || networkSettings.NetID == "" {
		return returnNode, errors.New("no network provided")
	}

	var err error

	var currentServers = GetServerNodes(networkSettings.NetID)
	var serverCount = 1
	if currentServers != nil {
		serverCount = len(currentServers) + 1
	}
	var ishub = "no"

	if networkSettings.IsPointToSite == "yes" {
		nodes, err := GetNetworkNodes(networkSettings.NetID)
		if err != nil || nodes == nil {
			ishub = "yes"
		} else {
			sethub := true
			for i := range nodes {
				if nodes[i].IsHub == "yes" {
					sethub = false
				}
			}
			if sethub {
				ishub = "yes"
			}
		}
	}
	var node = &models.Node{
		IsServer:        "yes",
		DNSOn:           "no",
		IsStatic:        "yes",
		Name:            fmt.Sprintf("%s-%d", models.NODE_SERVER_NAME, serverCount),
		MacAddress:      servercfg.GetNodeID(),
		ID:              "", // will be set to new uuid
		UDPHolePunch:    "no",
		IsLocal:         networkSettings.IsLocal,
		LocalRange:      networkSettings.LocalRange,
		OS:              runtime.GOOS,
		Version:         servercfg.Version,
		IsHub:           ishub,
		NetworkSettings: *networkSettings,
	}

	SetNodeDefaults(node)

	if servercfg.GetPlatform() == "Kubernetes" {
		node.ListenPort = KUBERNETES_LISTEN_PORT
		node.MTU = KUBERNETES_SERVER_MTU
	}

	if node.LocalRange != "" && node.LocalAddress == "" {
		logger.Log(1, "local vpn, getting local address from range:", networkSettings.LocalRange)
		node.LocalAddress, err = getServerLocalIP(networkSettings)
		if err != nil {
			node.LocalAddress = ""
			node.IsLocal = "no"
		}
	}

	if node.Endpoint == "" {
		if node.IsLocal == "yes" && node.LocalAddress != "" {
			node.Endpoint = node.LocalAddress
		} else {
			node.Endpoint, err = servercfg.GetPublicIP()
		}
		if err != nil || node.Endpoint == "" {
			logger.Log(0, "Error setting server node Endpoint.")
			return returnNode, err
		}
	}

	var privateKey = ""

	// Generate and set public/private WireGuard Keys
	if privateKey == "" {
		wgPrivatekey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			logger.Log(1, err.Error())
			return returnNode, err
		}
		privateKey = wgPrivatekey.String()
		node.PublicKey = wgPrivatekey.PublicKey().String()
	}

	node.Network = networkSettings.NetID

	logger.Log(2, "adding a server instance on network", node.Network)
	if err != nil {
		return returnNode, err
	}
	err = SetNetworkNodesLastModified(node.Network)
	if err != nil {
		return returnNode, err
	}

	// get free port based on returned default listen port
	node.ListenPort, err = ncutils.GetFreePort(node.ListenPort)
	if err != nil {
		logger.Log(2, "Error retrieving port:", err.Error())
	} else {
		logger.Log(1, "Set client port to", fmt.Sprintf("%d", node.ListenPort), "for network", node.Network)
	}

	// safety check. If returned node from server is local, but not currently configured as local, set to local addr
	if node.IsLocal == "yes" && node.LocalRange != "" {
		node.LocalAddress, err = ncutils.GetLocalIP(node.LocalRange)
		if err != nil {
			return returnNode, err
		}
		node.Endpoint = node.LocalAddress
	}

	if err = CreateNode(node); err != nil {
		return returnNode, err
	}
	if err = StorePrivKey(node.ID, privateKey); err != nil {
		return returnNode, err
	}

	peers, err := GetPeerUpdate(node)
	if err != nil && !ncutils.IsEmptyRecord(err) {
		logger.Log(1, "failed to retrieve peers")
		return returnNode, err
	}

	err = wireguard.InitWireguard(node, privateKey, peers.Peers)
	if err != nil {
		return returnNode, err
	}

	return *node, nil
}

// ServerUpdate - updates the server
// replaces legacy Checkin code
func ServerUpdate(serverNode *models.Node, ifaceDelta bool) error {
	if !IsLocalServer(serverNode) {
		logger.Log(1, "skipping server update as not the leader")
		return nil
	}

	var err = ServerPull(serverNode, ifaceDelta)
	if isDeleteError(err) {
		return DeleteNodeByID(serverNode, true)
	} else if err != nil && !ifaceDelta {
		err = ServerPull(serverNode, true)
		if err != nil {
			return err
		}
	}

	actionCompleted := checkNodeActions(serverNode)
	if actionCompleted == models.NODE_DELETE {
		return errors.New("node has been removed")
	}

	return serverPush(serverNode)
}

// == Private ==

func isDeleteError(err error) bool {
	return err != nil && strings.Contains(err.Error(), models.NODE_DELETE)
}

func checkNodeActions(node *models.Node) string {
	if node.Action == models.NODE_UPDATE_KEY {
		err := setWGKeyConfig(node)
		if err != nil {
			logger.Log(1, "unable to process reset keys request:", err.Error())
			return ""
		}
	}
	if node.Action == models.NODE_DELETE {
		err := DeleteNodeByID(node, true)
		if err != nil {
			logger.Log(1, "error deleting locally:", err.Error())
		}
		return models.NODE_DELETE
	}
	return ""
}

// == Private ==

// ServerPull - performs a server pull
func ServerPull(serverNode *models.Node, ifaceDelta bool) error {
	if serverNode.IsServer != "yes" {
		return fmt.Errorf("attempted pull from non-server node: %s - %s", serverNode.Name, serverNode.ID)
	}

	var err error
	if serverNode.IPForwarding == "yes" {
		if err = setIPForwardingLinux(); err != nil {
			return err
		}
	}
	serverNode.OS = runtime.GOOS

	if ifaceDelta {
		// check for interface change
		// checks if address is in use by another interface
		var oldIfaceName, isIfacePresent = isInterfacePresent(serverNode.Interface, serverNode.Address)
		if !isIfacePresent {
			if err = deleteInterface(oldIfaceName, serverNode.PostDown); err != nil {
				logger.Log(1, "could not delete old interface", oldIfaceName)
			}
			logger.Log(1, "removed old interface", oldIfaceName)
		}
		if err = setWGConfig(serverNode, false); err != nil {
			return err
		}
		// handle server side update
		if err = UpdateNode(serverNode, serverNode); err != nil {
			return err
		}
	} else {
		if err = setWGConfig(serverNode, true); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ServerPull(serverNode, true)
			} else {
				return err
			}
		}
	}

	return nil
}

func getServerLocalIP(networkSettings *models.Network) (string, error) {

	var networkCIDR = networkSettings.LocalRange
	var currentAddresses, _ = net.InterfaceAddrs()
	var _, currentCIDR, cidrErr = net.ParseCIDR(networkCIDR)
	if cidrErr != nil {
		logger.Log(1, "error on server local IP, invalid CIDR provided:", networkCIDR)
		return "", cidrErr
	}
	for _, addr := range currentAddresses {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if currentCIDR.Contains(ip) {
			logger.Log(1, "found local ip on network,", networkSettings.NetID, ", set to", ip.String())
			return ip.String(), nil
		}
	}
	return "", errors.New("could not find a local ip for server")
}

func serverPush(serverNode *models.Node) error {
	serverNode.OS = runtime.GOOS
	serverNode.SetLastCheckIn()
	return UpdateNode(serverNode, serverNode)
}
