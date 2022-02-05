package local

import (
	"net"

	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TODO handle ipv6 in future

// SetPeerRoutes - sets/removes ip routes for each peer on a network
func SetPeerRoutes(iface, currentNodeAddr string, oldPeers map[string][]net.IPNet, newPeers []wgtypes.PeerConfig) {
	// traverse through all recieved peers
	for _, peer := range newPeers {
		// if pubkey found in existing peers, check against existing peer
		currPeerAllowedIPs := oldPeers[peer.PublicKey.String()]
		if currPeerAllowedIPs != nil {
			// traverse IPs, check to see if old peer contains each IP
			for _, allowedIP := range peer.AllowedIPs { // compare new ones (if any) to old ones
				if !ncutils.IPNetSliceContains(currPeerAllowedIPs, allowedIP) {
					if err := setRoute(iface, &allowedIP, allowedIP.IP.String()); err != nil {
						ncutils.PrintLog(err.Error(), 1)
					}
				}
			}
			for _, allowedIP := range currPeerAllowedIPs { // compare old ones (if any) to new ones
				if !ncutils.IPNetSliceContains(peer.AllowedIPs, allowedIP) {
					if err := deleteRoute(iface, &allowedIP, allowedIP.IP.String()); err != nil {
						ncutils.PrintLog(err.Error(), 1)
					}
				}
			}
			delete(oldPeers, peer.PublicKey.String()) // remove peer as it was found and processed
		} else {
			for _, allowedIP := range peer.AllowedIPs { // add all routes as peer doesn't exist
				if err := setRoute(iface, &allowedIP, allowedIP.String()); err != nil {
					ncutils.PrintLog(err.Error(), 1)
				}
			}
		}
	}

	// traverse through all remaining existing peers
	for _, allowedIPs := range oldPeers {
		for _, allowedIP := range allowedIPs {
			deleteRoute(iface, &allowedIP, allowedIP.IP.String())
		}
	}
}

// SetCurrentPeerRoutes - sets all the current peers
func SetCurrentPeerRoutes(iface, currentAddr string, peers []wgtypes.Peer) {
	for _, peer := range peers {
		for _, allowedIP := range peer.AllowedIPs {
			setRoute(iface, &allowedIP, currentAddr)
		}
	}
}

// SetCIDRRoute - sets the CIDR route, used on join and restarts
func SetCIDRRoute(iface, currentAddr string, cidr *net.IPNet) {
	setCidr(iface, currentAddr, cidr)
}
