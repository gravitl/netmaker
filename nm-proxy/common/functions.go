package common

import (
	"github.com/gravitl/netmaker/nm-proxy/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func GetPeer(peerKey wgtypes.Key) (*models.Conn, bool) {
	var peerInfo *models.Conn
	var found bool
	peerInfo, found = WgIfaceMap.PeerMap[peerKey.String()]
	peerInfo.Mutex.RLock()
	defer peerInfo.Mutex.RUnlock()
	return peerInfo, found

}

func UpdatePeer(peer *models.Conn) {
	peer.Mutex.Lock()
	defer peer.Mutex.Unlock()
	WgIfaceMap.PeerMap[peer.Key.String()] = peer
}
