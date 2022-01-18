package serverctl

import (
	"fmt"

	"github.com/gravitl/netmaker/models"
)

// ServerQueue - holds data to be updated across the server
var ServerQueue chan models.ServerUpdateData

func init() {
	ServerQueue = make(chan models.ServerUpdateData, 100)
}

// Push - Pushes ServerUpdateData to be used later
func Push(serverData models.ServerUpdateData) {
	ServerQueue <- serverData
}

// Pop - fetches first available data from queue
func Pop() (models.ServerUpdateData, error) {
	select {
	case serverData := <-ServerQueue:
		return serverData, nil
	default:
		return models.ServerUpdateData{}, fmt.Errorf("empty server queue")
	}
}
