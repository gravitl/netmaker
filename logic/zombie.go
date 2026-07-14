package logic

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"
)

const (
	// ZOMBIE_TIMEOUT - timeout in hours for checking zombie status
	ZOMBIE_TIMEOUT = 6
	// ZOMBIE_DELETE_TIME - timeout in minutes for zombie node deletion
	ZOMBIE_DELETE_TIME = 10
)

var (
	// zombieChans and hostZombieChans map tenant ID -> that tenant's zombie-candidate channel.
	zombieChans     sync.Map
	hostZombieChans sync.Map
)

func zombieChan(tenantID string) chan uuid.UUID {
	ch, _ := zombieChans.LoadOrStore(tenantID, make(chan uuid.UUID, 10))
	return ch.(chan uuid.UUID)
}

func hostZombieChan(tenantID string) chan uuid.UUID {
	ch, _ := hostZombieChans.LoadOrStore(tenantID, make(chan uuid.UUID, 10))
	return ch.(chan uuid.UUID)
}

// CheckZombies - checks if new node has same hostid as existing node
// if so, existing node is added to zombie node quarantine list
// also cleans up nodes past their expiration date
func CheckZombies(ctx context.Context, _node *schema.Node) {
	nodes, err := GetNetworkNodes(ctx, _node.Network.Name)
	if err != nil {
		logger.Log(1, "Failed to retrieve network nodes", _node.Network.Name, err.Error())
		return
	}
	zombies := zombieChan(scope.ID(ctx))
	for _, node := range nodes {
		if node.ID.String() == _node.ID {
			//skip self
			continue
		}
		if node.HostID.String() == _node.HostID {
			logger.Log(0, "adding ", node.ID.String(), " to zombie list")
			zombies <- node.ID
		}
	}
}

// checkForZombieHosts - checks if new host has the same macAddress as an existing host
// if true, existing host is added to host zombie collection
func checkForZombieHosts(ctx context.Context, h *schema.Host) {
	hosts, err := (&schema.Host{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(3, "error retrieving all hosts", err.Error())
	}
	hostZombies := hostZombieChan(scope.ID(ctx))
	for _, existing := range hosts {
		if existing.ID == h.ID {
			//probably an unnecessary check as new host should not be in database yet, but just in case
			//skip self
			continue
		}
		if existing.MacAddress.String() == h.MacAddress.String() {
			//add to hostZombies
			hostZombies <- existing.ID
			//add all nodes belonging to host to zombile list
			for _, node := range existing.Nodes {
				id, err := uuid.Parse(node)
				if err != nil {
					logger.Log(3, "error parsing uuid from host.Nodes", err.Error())
					continue
				}
				hostZombies <- id
			}
		}
	}
}

// ManageZombies - lists tenants and starts a per-tenant zombie-management worker for each.
func ManageZombies(ctx context.Context) {
	logger.Log(2, "Zombie management started")
	tenants, err := (&schema.Tenant{}).ListAll(db.WithContext(ctx))
	if err != nil {
		logger.Log(1, "failed to list tenants for zombie management", err.Error())
	}
	for _, tenant := range tenants {
		tenantCtx := scope.WithContext(db.WithContext(ctx), scope.TenantScope, tenant.ID)
		go manageTenantZombies(tenantCtx)
	}
	<-ctx.Done()
	close(DeleteNodesCh)
}

// manageTenantZombies - goroutine which adds/removes/deletes nodes from a single tenant's
// zombie node quarantine list.
func manageTenantZombies(ctx context.Context) {
	tenantID := scope.ID(ctx)
	zombieCh := zombieChan(tenantID)
	hostZombieCh := hostZombieChan(tenantID)
	var zombies, hostZombies []uuid.UUID

	go initializeZombies(ctx)
	go checkPendingRemovalNodes(ctx)
	// Zombie Nodes Cleanup Four Times a Day
	ticker := time.NewTicker(time.Hour * ZOMBIE_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case id := <-zombieCh:
			zombies = append(zombies, id)
		case id := <-hostZombieCh:
			hostZombies = append(hostZombies, id)
		case <-ticker.C: // run this check 4 times a day
			logger.Log(3, "checking for zombie nodes", "tenant", tenantID)
			cleanupOrphanedNodes(ctx)
			if len(zombies) > 0 {
				for i := len(zombies) - 1; i >= 0; i-- {
					node, err := GetNodeByID(zombies[i].String())
					if err != nil {
						logger.Log(1, "error retrieving zombie node", zombies[i].String(), err.Error())
						logger.Log(1, "deleting ", node.ID.String(), " from zombie list")
						zombies = append(zombies[:i], zombies[i+1:]...)
						continue
					}
					if time.Since(node.LastCheckIn) > time.Minute*ZOMBIE_DELETE_TIME {
						if err := DeleteNode(ctx, &node, true); err != nil {
							logger.Log(1, "error deleting zombie node", zombies[i].String(), err.Error())
							continue
						}
						node.PendingDelete = true
						node.Action = schema.NODE_DELETE
						DeleteNodesCh <- &node
						logger.Log(1, "deleting zombie node", node.ID.String())
						zombies = append(zombies[:i], zombies[i+1:]...)
					}
				}
			}
			if len(hostZombies) > 0 {
				logger.Log(3, "checking host zombies")
				for i := len(hostZombies) - 1; i >= 0; i-- {
					host := &schema.Host{ID: hostZombies[i]}
					err := host.Get(ctx)
					if err != nil {
						logger.Log(1, "error retrieving zombie host", err.Error())
						logger.Log(1, "deleting ", host.ID.String(), " from zombie list")
						hostZombies = append(hostZombies[:i], hostZombies[i+1:]...)
						continue
					}
					if len(host.Nodes) == 0 {
						if err := RemoveHost(ctx, host, true); err != nil {
							logger.Log(0, "error deleting zombie host", host.ID.String(), err.Error())
						}
						hostZombies = append(hostZombies[:i], hostZombies[i+1:]...)
					}
				}
			}

		}
	}
}

// cleanupOrphanedNodes removes nodes whose host_id references a missing host record.
func cleanupOrphanedNodes(ctx context.Context) {
	nodes, err := GetAllNodes(ctx)
	if err != nil {
		logger.Log(1, "failed to retrieve nodes for orphan cleanup", err.Error())
		return
	}
	for _, n := range nodes {
		node := n
		if node.PendingDelete || node.Action == schema.NODE_DELETE {
			continue
		}
		host := &schema.Host{ID: node.HostID}
		if err := host.Get(ctx); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Log(0, "error checking host for orphaned node", node.ID.String(), err.Error())
				continue
			}
			logger.Log(0, "orphaned node found (no host record), deleting", node.ID.String())
			if err := DeleteNode(ctx, &node, true); err != nil {
				logger.Log(0, "error deleting orphaned node", node.ID.String(), err.Error())
				continue
			}
		}
	}
}

func checkPendingRemovalNodes(ctx context.Context) {
	nodes, _ := GetAllNodes(ctx)
	for _, node := range nodes {
		node := node
		pendingDelete := node.PendingDelete || node.Action == schema.NODE_DELETE
		if pendingDelete {
			DeleteNode(ctx, &node, true)
			DeleteNodesCh <- &node
			continue
		}
	}
}

// initializeZombies - populates the zombie quarantine list (should be called from initialization)
func initializeZombies(ctx context.Context) {
	nodes, err := GetAllNodes(ctx)
	if err != nil {
		logger.Log(1, "failed to retrieve nodes", err.Error())
		return
	}
	zombies := zombieChan(scope.ID(ctx))
	for _, node := range nodes {
		othernodes, err := GetNetworkNodes(ctx, node.Network)
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
					zombies <- othernode.ID
					logger.Log(1, "adding", othernode.ID.String(), "to zombie list")
				} else {
					zombies <- node.ID
					logger.Log(1, "adding", node.ID.String(), "to zombie list")
				}
			}
		}
	}
	cleanupOrphanedNodes(ctx)
}
