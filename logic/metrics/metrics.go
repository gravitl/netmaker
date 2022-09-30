package metrics

import (
	"runtime"
	"time"

	"github.com/go-ping/ping"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Collect - collects metrics
func Collect(iface string, peerMap models.PeerMap) (*models.Metrics, error) {
	var metrics models.Metrics
	metrics.Connectivity = make(map[string]models.Metric)
	var wgclient, err = wgctrl.New()
	if err != nil {
		fillUnconnectedData(&metrics, peerMap)
		return &metrics, err
	}
	defer wgclient.Close()

	if runtime.GOOS == "darwin" {
		iface, err = wireguard.GetRealIface(iface)
		if err != nil {
			fillUnconnectedData(&metrics, peerMap)
			return &metrics, err
		}
	}
	device, err := wgclient.Device(iface)
	if err != nil {
		fillUnconnectedData(&metrics, peerMap)
		return &metrics, err
	}
	// TODO handle freebsd??
	for i := range device.Peers {
		currPeer := device.Peers[i]
		id := peerMap[currPeer.PublicKey.String()].ID
		address := peerMap[currPeer.PublicKey.String()].Address
		if id == "" || address == "" {
			logger.Log(0, "attempted to parse metrics for invalid peer from server", id, address)
			continue
		}
		var newMetric = models.Metric{
			NodeName: peerMap[currPeer.PublicKey.String()].Name,
			IsServer: peerMap[currPeer.PublicKey.String()].IsServer,
		}
		logger.Log(2, "collecting metrics for peer", address)
		newMetric.TotalReceived = currPeer.ReceiveBytes
		newMetric.TotalSent = currPeer.TransmitBytes

		// get latency
		pinger, err := ping.NewPinger(address)
		if err != nil {
			logger.Log(0, "could not initiliaze ping for metrics on peer address", address, err.Error())
			newMetric.Connected = false
			newMetric.Latency = 999
		} else {
			pinger.Count = 1
			pinger.Timeout = time.Second * 2
			err = pinger.Run()
			if err != nil {
				logger.Log(0, "failed ping for metrics on peer address", address, err.Error())
				newMetric.Connected = false
				newMetric.Latency = 999
			} else {
				pingStats := pinger.Statistics()
				if pingStats.PacketsRecv > 0 {
					newMetric.Uptime = 1
					newMetric.Connected = true
					newMetric.Latency = pingStats.AvgRtt.Milliseconds()
				}
			}
		}

		// check device peer to see if WG is working if ping failed
		if !newMetric.Connected {
			if currPeer.ReceiveBytes > 0 &&
				currPeer.TransmitBytes > 0 &&
				time.Now().Before(currPeer.LastHandshakeTime.Add(time.Minute<<1)) {
				newMetric.Connected = true
				newMetric.Uptime = 1
			}
		}

		newMetric.TotalTime = 1
		metrics.Connectivity[id] = newMetric
	}

	fillUnconnectedData(&metrics, peerMap)
	return &metrics, nil
}

// GetExchangedBytesForNode - get exchanged bytes for current node peers
func GetExchangedBytesForNode(node *models.Node, metrics *models.Metrics) error {

	peers, err := logic.GetPeerUpdate(node)
	if err != nil {
		logger.Log(0, "Failed to get peers: ", err.Error())
		return err
	}
	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()
	device, err := wgclient.Device(node.Interface)
	if err != nil {
		return err
	}
	for _, currPeer := range device.Peers {
		id := peers.PeerIDs[currPeer.PublicKey.String()].ID
		address := peers.PeerIDs[currPeer.PublicKey.String()].Address
		if id == "" || address == "" {
			logger.Log(0, "attempted to parse metrics for invalid peer from server", id, address)
			continue
		}
		logger.Log(2, "collecting exchanged bytes info for peer: ", address)
		peerMetric := metrics.Connectivity[id]
		peerMetric.TotalReceived = currPeer.ReceiveBytes
		peerMetric.TotalSent = currPeer.TransmitBytes
		metrics.Connectivity[id] = peerMetric
	}
	return nil
}

// == used to fill zero value data for non connected peers ==
func fillUnconnectedData(metrics *models.Metrics, peerMap models.PeerMap) {
	for r := range peerMap {
		id := peerMap[r].ID
		if !metrics.Connectivity[id].Connected {
			newMetric := models.Metric{
				NodeName:  peerMap[r].Name,
				IsServer:  peerMap[r].IsServer,
				Uptime:    0,
				TotalTime: 1,
				Connected: false,
				Latency:   999,
				PercentUp: 0,
			}
			metrics.Connectivity[id] = newMetric
		}
	}
}
