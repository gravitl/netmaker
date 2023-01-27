package logic

import (
	"context"
	"net"
	"time"

	"github.com/google/uuid"
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
	zombies      []uuid.UUID
	removeZombie chan uuid.UUID = make(chan (uuid.UUID), 10)
	newZombie    chan uuid.UUID = make(chan (uuid.UUID), 10)
)

// CheckZombies - checks if new node has same macaddress as existing node
// if so, existing node is added to zombie node quarantine list
// also cleans up nodes past their expiration date
func CheckZombies(newnode *models.Node, mac net.HardwareAddr) {
	nodes, err := GetNetworkNodes(newnode.Network)
	if err != nil {
		logger.Log(1, "Failed to retrieve network nodes", newnode.Network, err.Error())
		return
	}
	for _, node := range nodes {
		host, err := GetHost(node.HostID.String())
		if err != nil {
			// should we delete the node if host not found ??
			continue
		}
		if host.MacAddress.String() == mac.String() || time.Now().After(node.ExpirationDateTime) {
			logger.Log(0, "adding ", node.ID.String(), " to zombie list")
			newZombie <- node.ID
		}
	}
}

// ManageZombies - goroutine which adds/removes/deletes nodes from the zombie node quarantine list
func ManageZombies(ctx context.Context, peerUpdate chan *models.Node) {
	logger.Log(2, "Zombie management started")
	InitializeZombies()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-newZombie:
			logger.Log(1, "adding", id.String(), "to zombie quaratine list")
			zombies = append(zombies, id)
		case id := <-removeZombie:
			found := false
			if len(zombies) > 0 {
				for i := len(zombies) - 1; i >= 0; i-- {
					if zombies[i] == id {
						logger.Log(1, "removing zombie from quaratine list", zombies[i].String())
						zombies = append(zombies[:i], zombies[i+1:]...)
						found = true
					}
				}
			}
			if !found {
				logger.Log(3, "no zombies found")
			}
		case <-time.After(time.Second * ZOMBIE_TIMEOUT):
			logger.Log(3, "checking for zombie nodes")
			if len(zombies) > 0 {
				for i := len(zombies) - 1; i >= 0; i-- {
					node, err := GetNodeByID(zombies[i].String())
					if err != nil {
						logger.Log(1, "error retrieving zombie node", zombies[i].String(), err.Error())
						logger.Log(1, "deleting ", node.ID.String(), " from zombie list")
						zombies = append(zombies[:i], zombies[i+1:]...)
						continue
					}
					if time.Since(node.LastCheckIn) > time.Minute*ZOMBIE_DELETE_TIME || time.Now().After(node.ExpirationDateTime) {
						if err := DeleteNode(&node, true); err != nil {
							logger.Log(1, "error deleting zombie node", zombies[i].String(), err.Error())
							continue
						}
						node.Action = models.NODE_DELETE
						peerUpdate <- &node
						logger.Log(1, "deleting zombie node", node.ID.String())
						zombies = append(zombies[:i], zombies[i+1:]...)
					}
				}
			}
		}
	}
}

// InitializeZombies - populates the zombie quarantine list (should be called from initialization)
func InitializeZombies() {
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
			if node.HostID == othernode.HostID {
				if node.LastCheckIn.After(othernode.LastCheckIn) {
					zombies = append(zombies, othernode.ID)
					logger.Log(1, "adding", othernode.ID.String(), "to zombie list")
				} else {
					zombies = append(zombies, node.ID)
					logger.Log(1, "adding", node.ID.String(), "to zombie list")
				}
			}
		}
	}
}
