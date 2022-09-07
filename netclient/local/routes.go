package local

import (
	"fmt"
	"net"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TODO handle ipv6 in future

// SetPeerRoutes - sets/removes ip routes for each peer on a network
func SetPeerRoutes(iface string, oldPeers map[string]bool, newPeers []wgtypes.PeerConfig) {

	// get the default route
	var hasRoute bool
	gwIP, gwIface, err := GetDefaultRoute()
	if err != nil {
		logger.Log(0, "error getting default route:", err.Error())
	}
	if gwIP != "" && gwIface != "" && err == nil {
		hasRoute = true
	}

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
		if peer.Endpoint == nil {
			continue
		}
		if hasRoute && !ncutils.IpIsPrivate(peer.Endpoint.IP) {
			ipNet, err := ncutils.GetIPNetFromString(peer.Endpoint.IP.String())
			if err != nil {
				logger.Log(0, "error parsing ip:", err.Error())
			}
			SetExplicitRoute(gwIface, &ipNet, gwIP)
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

	// get the default route
	var hasRoute bool
	gwIP, gwIface, err := GetDefaultRoute()
	if err != nil {
		logger.Log(0, "error getting default route:", err.Error())
	}
	if gwIP != "" && gwIface != "" && err == nil {
		hasRoute = true
	}

	// traverse through all recieved peers
	for _, peer := range peers {
		for _, allowedIP := range peer.AllowedIPs {
			setRoute(iface, &allowedIP, currentAddr)
		}
		if peer.Endpoint == nil {
			continue
		}
		if hasRoute && !ncutils.IpIsPrivate(peer.Endpoint.IP) {
			ipNet, err := ncutils.GetIPNetFromString(peer.Endpoint.IP.String())
			if err != nil {
				logger.Log(0, "error parsing ip:", err.Error())
			}
			SetExplicitRoute(gwIface, &ipNet, gwIP)
		}
	}

}

// FlushPeerRoutes - removes all current peer routes
func FlushPeerRoutes(iface, currentAddr string, peers []wgtypes.Peer) {
	// get the default route
	var hasRoute bool
	gwIP, gwIface, err := GetDefaultRoute()
	if err != nil {
		logger.Log(0, "error getting default route:", err.Error())
	}
	if gwIP != "" && gwIface != "" && err == nil {
		hasRoute = true
	}

	for _, peer := range peers {
		for _, allowedIP := range peer.AllowedIPs {
			deleteRoute(iface, &allowedIP, currentAddr)
		}
		if peer.Endpoint == nil {
			continue
		}
		if hasRoute && !ncutils.IpIsPrivate(peer.Endpoint.IP) {
			ipNet, err := ncutils.GetIPNetFromString(peer.Endpoint.IP.String())
			if err != nil {
				logger.Log(0, "error parsing ip:", err.Error())
			}
			deleteRoute(gwIface, &ipNet, gwIP)
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

// SetNetmakerDomainRoute - sets explicit route over Gateway for a given DNS name
func SetNetmakerDomainRoute(domainRaw string) error {
	parts := strings.Split(domainRaw, ":")
	hostname := parts[0]
	var address net.IPNet

	gwIP, gwIface, err := GetDefaultRoute()
	if err != nil {
		return fmt.Errorf("error getting default route: %w", err)
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			address, err = ncutils.GetIPNetFromString(ipv4.String())
			if err == nil {
				break
			}
		}
	}
	if err != nil || address.IP == nil {
		return fmt.Errorf("address not found")
	}
	return SetExplicitRoute(gwIface, &address, gwIP)
}
