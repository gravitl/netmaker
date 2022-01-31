package ncutils

import (
	"log"

	"github.com/gravitl/netmaker/models"
)

func IfaceDelta(currentNode *models.Node, newNode *models.Node) bool {
	// single comparison statements
	log.Println("DELETE: checking stuff")
	if newNode.Endpoint != currentNode.Endpoint ||
		newNode.LocalAddress != currentNode.LocalAddress ||
		newNode.PublicKey != currentNode.PublicKey ||
		newNode.Address != currentNode.Address ||
		newNode.IsEgressGateway != currentNode.IsEgressGateway ||
		newNode.IsIngressGateway != currentNode.IsIngressGateway ||
		newNode.IsRelay != currentNode.IsRelay ||
		newNode.UDPHolePunch != currentNode.UDPHolePunch ||
		newNode.IsPending != currentNode.IsPending ||
		newNode.PersistentKeepalive != currentNode.PersistentKeepalive ||
		len(newNode.ExcludedAddrs) != len(currentNode.ExcludedAddrs) ||
		len(newNode.AllowedIPs) != len(currentNode.AllowedIPs) {
		return true
	}

	// multi-comparison statements
	if newNode.IsDualStack == "yes" {
		if newNode.Address6 != currentNode.Address6 {
			return true
		}
	}

	if newNode.IsEgressGateway == "yes" {
		if len(currentNode.EgressGatewayRanges) != len(newNode.EgressGatewayRanges) {
			return true
		}
		for _, address := range newNode.EgressGatewayRanges {
			if !StringSliceContains(currentNode.EgressGatewayRanges, address) {
				return true
			}
		}
	}

	if newNode.IsRelay == "yes" {
		if len(currentNode.RelayAddrs) != len(newNode.RelayAddrs) {
			return true
		}
		for _, address := range newNode.RelayAddrs {
			if !StringSliceContains(currentNode.RelayAddrs, address) {
				return true
			}
		}
	}

	for _, address := range newNode.AllowedIPs {
		if !StringSliceContains(currentNode.AllowedIPs, address) {
			return true
		}
	}
	log.Println("DELETE: guess it's false")
	return false
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
