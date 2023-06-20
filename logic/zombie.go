package logic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

const (
	// ZOMBIE_TIMEOUT - timeout in hours for checking zombie status
	ZOMBIE_TIMEOUT = 6
	// ZOMBIE_DELETE_TIME - timeout in minutes for zombie node deletion
	ZOMBIE_DELETE_TIME = 10
)

var (
	zombies       []uuid.UUID
	hostZombies   []uuid.UUID
	newZombie     chan uuid.UUID = make(chan (uuid.UUID), 10)
	newHostZombie chan uuid.UUID = make(chan (uuid.UUID), 10)
)

// CheckZombies - checks if new node has same hostid as existing node
// if so, existing node is added to zombie node quarantine list
// also cleans up nodes past their expiration date
func CheckZombies(newnode *models.Node) {
	nodes, err := GetNetworkNodes(newnode.Network)
	if err != nil {
		logger.Log(1, "Failed to retrieve network nodes", newnode.Network, err.Error())
		return
	}
	for _, node := range nodes {
		if node.ID == newnode.ID {
			//skip self
			continue
		}
		if node.HostID == newnode.HostID || time.Now().After(node.ExpirationDateTime) {
			logger.Log(0, "adding ", node.ID.String(), " to zombie list")
			newZombie <- node.ID
		}
	}
}

// checkForZombieHosts - checks if new host has the same macAddress as an existing host
// if true, existing host is added to host zombie collection
func checkForZombieHosts(h *models.Host) {
	hosts, err := GetAllHosts()
	if err != nil {
		logger.Log(3, "errror retrieving all hosts", err.Error())
	}
	for _, existing := range hosts {
		if existing.ID == h.ID {
			//probably an unnecessary check as new host should not be in database yet, but just in case
			//skip self
			continue
		}
		if existing.MacAddress.String() == h.MacAddress.String() {
			//add to hostZombies
			newHostZombie <- existing.ID
			//add all nodes belonging to host to zombile list
			for _, node := range existing.Nodes {
				id, err := uuid.Parse(node)
				if err != nil {
					logger.Log(3, "error parsing uuid from host.Nodes", err.Error())
					continue
				}
				newHostZombie <- id
			}
		}
	}
}

// ManageZombies - goroutine which adds/removes/deletes nodes from the zombie node quarantine list
func ManageZombies(ctx context.Context, peerUpdate chan *models.Node) {
	logger.Log(2, "Zombie management started")
	CheckAllZombies()

	// run this check 4 times a day
	duration := time.Hour * ZOMBIE_TIMEOUT
	delay := time.NewTimer(duration)
	for {
		select {
		case <-ctx.Done():
			close(peerUpdate)
			return
		case id := <-newZombie:
			zombies = append(zombies, id)
		case id := <-newHostZombie:
			hostZombies = append(hostZombies, id)
		case <-delay.C:
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
			if len(hostZombies) > 0 {
				logger.Log(3, "checking host zombies")
				for i := len(hostZombies) - 1; i >= 0; i-- {
					host, err := GetHost(hostZombies[i].String())
					if err != nil {
						logger.Log(1, "error retrieving zombie host", err.Error())
						logger.Log(1, "deleting ", host.ID.String(), " from zombie list")
						zombies = append(zombies[:i], zombies[i+1:]...)
						continue
					}
					if len(host.Nodes) == 0 {
						if err := RemoveHost(host); err != nil {
							logger.Log(0, "error deleting zombie host", host.ID.String(), err.Error())
						}
					}
				}
			}
			delay.Reset(duration)
		}
	}
}

// CheckAllZombies - populates the zombie quarantine list (should be called from initialization)
func CheckAllZombies() {
	nodes, err := GetAllNodes()
	if err != nil {
		logger.Log(1, "failed to retrieve nodes", err.Error())
		return
	}
	index := map[string][]models.Node{}
	for _, node := range nodes {
		if _, ok := index[node.Network]; !ok {
			index[node.Network] = []models.Node{}
		}
		index[node.Network] = append(index[node.Network], node)
	}
	for i := range index {
		net := index[i]
		for ii := range net {
			node1 := net[ii]
			for iii := range net {
				node2 := net[iii]
				if node1.ID == node2.ID {
					continue
				}
				if node1.HostID == node2.HostID {
					if node1.LastCheckIn.After(node2.LastCheckIn) {
						newZombie <- node2.ID
						logger.Log(1, "adding", node2.ID.String(), "to zombie list")
					} else {
						newZombie <- node1.ID
						logger.Log(1, "adding", node1.ID.String(), "to zombie list")
					}
				}
			}
		}
	}
}
