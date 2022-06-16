package local

import (
	"net"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TODO handle ipv6 in future

// SetPeerRoutes - sets/removes ip routes for each peer on a network
func SetPeerRoutes(iface string, oldPeers map[string]bool, newPeers []wgtypes.PeerConfig) {
	// traverse through all recieved peers
	for _, peer := range newPeers {
		for _, allowedIP := range peer.AllowedIPs {
			if !oldPeers[allowedIP.String()] {
				if err := setRoute(iface, &allowedIP, allowedIP.IP.String()); err != nil {
					logger.Log(1, err.Error())
				}
			} else {
				delete(oldPeers, allowedIP.String())
			}
		}
	}
	// traverse through all remaining existing peers
	for i := range oldPeers {
		ip, err := ncutils.GetIPNetFromString(i)
		if err != nil {
			logger.Log(1, err.Error())
		} else {
			deleteRoute(iface, &ip, ip.IP.String())
		}
	}
}

// SetCurrentPeerRoutes - sets all the current peers
func SetCurrentPeerRoutes(iface, currentAddr string, peers []wgtypes.PeerConfig) {
	for _, peer := range peers {
		for _, allowedIP := range peer.AllowedIPs {
			setRoute(iface, &allowedIP, currentAddr)
		}
	}
}

// FlushPeerRoutes - removes all current peer routes
func FlushPeerRoutes(iface, currentAddr string, peers []wgtypes.Peer) {
	for _, peer := range peers {
		for _, allowedIP := range peer.AllowedIPs {
			deleteRoute(iface, &allowedIP, currentAddr)
		}
	}
}

// SetCIDRRoute - sets the CIDR route, used on join and restarts
func SetCIDRRoute(iface, currentAddr string, cidr *net.IPNet) {
	setCidr(iface, currentAddr, cidr)
}

// RemoveCIDRRoute - removes a static cidr route
func RemoveCIDRRoute(iface, currentAddr string, cidr *net.IPNet) {
	removeCidr(iface, cidr, currentAddr)
}
