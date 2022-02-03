//go:build linux
// +build linux

package local

import (
	//"github.com/davecgh/go-spew/spew"

	"net"

	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func routeExists(iface, address, mask string) bool {
	return false
}

func setRoute(iface, address, mask string) error {
	_, err := ncutils.RunCmd("ip route add", true)
	return err
}

func DeleteRoute(iface, address string) error {
	var err error
	return err
}

func ApplyRoutesFromConf(confPath string) error {
	var err error
	return err
}

//func SetLinuxPeerRoutes(currentPeers []wgtypes.PeerConfig, newPeers []wgtypes.PeerConfig) {
func SetLinuxPeerRoutes(iface string, oldPeers map[string][]net.IP, newPeers []wgtypes.PeerConfig) {

	// traverse through all recieved peers
	for _, peer := range newPeers {
		// if pubkey found in existing peers, check against existing peer
		currPeerAllowedIPs := oldPeers[peer.PublicKey.String()]
		if currPeerAllowedIPs != nil {
			// traverse IPs, check to see if old peer contains each IP
			for _, allowedIP := range peer.AllowedIPs {
				if !ncutils.StringSliceContains(currPeerAllowedIPs, allowedIP.IP.String()) {
					if err := setRoute(iface, allowedIP.IP.String(), allowedIP.Mask.String()); err != nil {
						ncutils.PrintLog(err.Error(), 1)
					}
				}
			}
			for _, allowedIP := range currPeerAllowedIPs {
				if !ncutils.StringSliceContains(currPeerAllowedIPs, allowedIP) {
					if err := setRoute(iface, allowedIP.IP.String(), allowedIP.Mask.String()); err != nil {
						ncutils.PrintLog(err.Error(), 1)
					}
				}
			}
			delete(oldPeers, peer.PublicKey.String())
		} else {
			for _, allowedIP := range peer.AllowedIPs {
				if err := setRoute(iface, allowedIP.IP.String(), allowedIP.Mask.String()); err != nil {
					ncutils.PrintLog(err.Error(), 1)
				}
			}
		}
	}

	// traverse through all existing peers
	for _, peer := range oldPeers {
		// if pubkey found in existing peers, check against existing peer
		currPeerAllowedIPs := oldPeers[peer.PublicKey.String()]
		if currPeerAllowedIPs != nil {
			// traverse IPs, check to see if old peer contains each IP
			for _, allowedIP := range peer.AllowedIPs {
				if !ncutils.StringSliceContains(currPeerAllowedIPs, allowedIP.IP.String()) {
					if err := setRoute(iface, allowedIP.IP.String(), allowedIP.Mask.String()); err != nil {
						ncutils.PrintLog(err.Error(), 1)
					}
				}
			}
		} else {
			for _, allowedIP := range peer.AllowedIPs {
				if err := setRoute(iface, allowedIP.IP.String(), allowedIP.Mask.String()); err != nil {
					ncutils.PrintLog(err.Error(), 1)
				}
			}
		}
	}

	// delete removed AllowedIPs
	/*
		for _, currentPeer := range currentPeers {
			for _, oldIP := range currentPeer.AllowedIPs {
				found := true
				for _, newPeer := range newPeers {
					for _, newIP := range newPeer.AllowedIPs {
						if
					}
				}
			}
		}
	*/
}

func GetCurrentIPs() []string {
	client, err := wgctrl.New()
	if err != nil {
		ncutils.PrintLog("failed to start wgctrl", 0)
		return err
	}
	defer client.Close()
	device, err := client.Device(iface)
	if err != nil {
		ncutils.PrintLog("failed to parse interface", 0)
		return err
	}
	devicePeers = device.Peers
}
