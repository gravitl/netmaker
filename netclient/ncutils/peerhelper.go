package ncutils

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetPeers - gets the peers from a given WireGuard interface
func GetPeers(iface string) ([]wgtypes.Peer, error) {

	var peers []wgtypes.Peer
	output, err := RunCmd("wg show "+iface+" dump", true)
	if err != nil {
		return peers, err
	}
	for i, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if i == 0 {
			continue
		}
		var allowedIPs []net.IPNet
		fields := strings.Fields(line)
		if len(fields) < 4 {
			logger.Log(0, "error parsing peer: "+line)
			continue
		}
		pubkeystring := fields[0]
		endpointstring := fields[2]
		allowedipstring := fields[3]
		var pkeepalivestring string
		if len(fields) > 7 {
			pkeepalivestring = fields[7]
		}
		// AllowedIPs = private IP + defined networks

		pubkey, err := wgtypes.ParseKey(pubkeystring)
		if err != nil {
			logger.Log(0, "error parsing peer key "+pubkeystring)
			continue
		}
		ipstrings := strings.Split(allowedipstring, ",")
		for _, ipstring := range ipstrings {
			var netip net.IP
			if netip = net.ParseIP(strings.Split(ipstring, "/")[0]); netip != nil {
				allowedIPs = append(
					allowedIPs,
					net.IPNet{
						IP:   netip,
						Mask: netip.DefaultMask(),
					},
				)
			}
		}
		if len(allowedIPs) == 0 {
			logger.Log(0, "error parsing peer "+pubkeystring+", no allowedips found")
			continue
		}
		var endpointarr []string
		var endpointip net.IP
		if endpointarr = strings.Split(endpointstring, ":"); len(endpointarr) != 2 {
			logger.Log(0, "error parsing peer "+pubkeystring+", could not parse endpoint: "+endpointstring)
			continue
		}
		if endpointip = net.ParseIP(endpointarr[0]); endpointip == nil {
			logger.Log(0, "error parsing peer "+pubkeystring+", could not parse endpoint: "+endpointarr[0])
			continue
		}
		var port int
		if port, err = strconv.Atoi(endpointarr[1]); err != nil {
			logger.Log(0, "error parsing peer "+pubkeystring+", could not parse port: "+err.Error())
			continue
		}
		var endpoint = net.UDPAddr{
			IP:   endpointip,
			Port: port,
		}
		var dur time.Duration
		if pkeepalivestring != "" {
			if dur, err = time.ParseDuration(pkeepalivestring + "s"); err != nil {
				logger.Log(0, "error parsing peer "+pubkeystring+", could not parse keepalive: "+err.Error())
			}
		}

		peers = append(peers, wgtypes.Peer{
			PublicKey:                   pubkey,
			Endpoint:                    &endpoint,
			AllowedIPs:                  allowedIPs,
			PersistentKeepaliveInterval: dur,
		})
	}

	return peers, err
}
