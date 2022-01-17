package serverctl

import (
	"fmt"

	"github.com/gravitl/netmaker/models"
)

// ServerQueue - holds data to be updated across the server
var ServerQueue chan ServerUpdateData

func init() {
	ServerQueue = make(chan ServerUpdateData, 100)
}

// ServerUpdateData - contains data to configure server
// and if it should set peers
type ServerUpdateData struct {
	UpdatePeers bool        `json:"updatepeers" bson:"updatepeers"`
	ServerNode  models.Node `json:"servernode" bson:"servernode"`
}

// Push - Pushes ServerUpdateData to be used later
func Push(serverData ServerUpdateData) {
	ServerQueue <- serverData
}

// Pop - fetches first available data from queue
func Pop() (ServerUpdateData, error) {
	select {
	case serverData := <-ServerQueue:
		return serverData, nil
	default:
		return ServerUpdateData{}, fmt.Errorf("empty server queue")
	}
}
