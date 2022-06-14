package logic

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

const (
	// ZOMBIE_TIMEOUT - timeout in seconds for checking zombie status
	ZOMBIE_TIMEOUT = 60
	// ZOMBIE_DELETE_TIME - timeout in minutes for zombie node deletion
	ZOMBIE_DELETE_TIME = 10
)

var (
	zombies      []string
	removeZombie chan string = make(chan (string))
	newZombie    chan string = make(chan (string))
)

// CheckZombies - checks if new node has same macaddress as existing node
// if so, existing node is added to zombie node quarantine list
func CheckZombies(newnode *models.Node) {
	nodes, err := GetNetworkNodes(newnode.Network)
	if err != nil {
		logger.Log(1, "Failed to retrieve network nodes", newnode.Network, err.Error())
		return
	}
	for _, node := range nodes {
		if node.MacAddress == newnode.MacAddress {
			newZombie <- node.ID
		}
	}
}

// ManageZombies - goroutine which adds/removes/deletes nodes from the zombie node quarantine list
func ManageZombies(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-newZombie:
			logger.Log(1, "adding", id, "to zombie quaratine list")
			zombies = append(zombies, id)
		case id := <-removeZombie:
			found := false
			for i, zombie := range zombies {
				if zombie == id {
					logger.Log(1, "removing zombie from quaratine list", zombie)
					zombies = append(zombies[:i], zombies[i+1:]...)
					found = true
				}
			}
			if !found {
				logger.Log(3, "no zombies found")
			}
		case <-time.After(time.Second * ZOMBIE_TIMEOUT):
			for i, zombie := range zombies {
				node, err := GetNodeByID(zombie)
				if err != nil {
					logger.Log(1, "error retrieving zombie node", zombie, err.Error())
					continue
				}
				if time.Since(time.Unix(node.LastCheckIn, 0)) > time.Minute*ZOMBIE_DELETE_TIME {
					if err := DeleteNodeByID(&node, true); err != nil {
						logger.Log(1, "error deleting zombie node", zombie, err.Error())
						continue
					}
					logger.Log(1, "deleting zombie node", node.Name)
					zombies = append(zombies[:i], zombies[i+1:]...)
				}
			}
		}
	}
}

// InitializeZombies - populates the zombie quarantine list (should be called from initialization)
func InitalizeZombies() {
	nodes, err := GetAllNodes()
	if err != nil {
		logger.Log(1, "failed to retrieve nodes", err.Error())
		return
	}
	for _, node := range nodes {
		othernodes, err := GetNetworkNodes(node.Network)
		if err != nil {
			logger.Log(1, "failled to retrieve nodes for network", node.Network, err.Error())
			continue
		}
		for _, othernode := range othernodes {
			if node.ID == othernode.ID {
				continue
			}
			if node.MacAddress == othernode.MacAddress {
				if node.LastCheckIn > othernode.LastCheckIn {
					zombies = append(zombies, othernode.ID)
				} else {
					zombies = append(zombies, node.ID)
				}
			}
		}
	}
}
