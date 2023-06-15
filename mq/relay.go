package mq

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

// PubPeerUpdate publishes a peer update to the client
// relay is set to a newly created relay node or nil for other peer updates
func PubPeerUpdate(client *models.Client) {
	peers := logic.GetPeerUpdate(&client.Host)
	p := models.PeerAction{
		Action: models.UpdatePeer,
		Peers:  peers,
	}
	if len(p.Peers) == 0 {
		slog.Info("no peer update for host", "host", client.Host.Name)
		return
	}
	data, err := json.Marshal(p)
	if err != nil {
		slog.Error("marshal peer update", "error", err)
		return
	}
	publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)
}
