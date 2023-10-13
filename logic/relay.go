package logic

import (
	"github.com/gravitl/netmaker/models"
	"net"
)

var GetRelays = func() ([]models.Node, error) {
	return []models.Node{}, nil
}

var RelayedAllowedIPs = func(peer, node *models.Node) []net.IPNet {
	return []net.IPNet{}
}

var GetAllowedIpsForRelayed = func(relayed, relay *models.Node) []net.IPNet {
	return []net.IPNet{}
}

var UpdateRelayed = func(currentNode, newNode *models.Node) {
}

var SetRelayedNodes = func(setRelayed bool, relay string, relayed []string) []models.Node {
	return []models.Node{}
}

var RelayUpdates = func(currentNode, newNode *models.Node) bool {
	return false
}
