package ncutils

import (
	"strconv"
	"strings"
	"bufio"
	"net"
	"time"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func GetPeers(iface string) ([]wgtypes.Peer, error) {
	
	var peers []wgtypes.Peer
	
	output, err := RunCmd("wg show "+iface+" dump",true)
	if err != nil {
		return peers, err
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			Log("err reading standard input:"+ err.Error())
			return peers, err
		}
		var allowedIPs []net.IPNet
		fields := strings.Fields(scanner.Text())
		pubkeystring := fields[0]
		endpointstring := fields[1]
		allowedipstring := fields[3]
		pkeepalivestring := fields[7]
		// AllowedIPs = private IP + defined networks

		pubkey, err := wgtypes.ParseKey(pubkeystring)
		if err != nil {
			Log("error parsing peer key "+pubkeystring)
			continue
		}
		ipstrings := strings.Split(allowedipstring, ",")
		for _, ipstring := range ipstrings {
			var netip net.IP
			if netip = net.ParseIP(ipstring); netip != nil {
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
			Log("error parsing peer "+pubkeystring+", no allowedips found")
			continue
		}
		var endpointarr []string
		var endpointip net.IP
		if endpointarr = strings.Split(endpointstring,":"); len(endpointarr) != 2 {
			Log("error parsing peer "+pubkeystring+", could not parse endpoint: "+endpointstring)
			continue
		}
		if endpointip = net.ParseIP(endpointarr[0]); endpointip == nil {
			Log("error parsing peer "+pubkeystring+", could not parse endpoint: "+endpointarr[0])
			continue
		}
		var port int
		if port, err = strconv.Atoi(endpointarr[1]); err != nil {
			Log("error parsing peer "+pubkeystring+", could not parse port: "+err.Error())
			continue
		}
		var endpoint = net.UDPAddr {
			IP: endpointip,
			Port: port,
		}
		var dur time.Duration
		if pkeepalivestring != "" {
			if dur, err = time.ParseDuration(pkeepalivestring+"s"); err != nil {
				Log("error parsing peer "+pubkeystring+", could not parse keepalive: "+err.Error())
			}
		}


		peers = append(peers, wgtypes.Peer{
			PublicKey:         pubkey,
			Endpoint:          &endpoint,
			AllowedIPs:        allowedIPs,
			PersistentKeepaliveInterval: dur,
		})
	}

	return peers, err
}