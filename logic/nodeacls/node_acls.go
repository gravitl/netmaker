package nodeacls

import (
	"context"
	"errors"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/gorm"
)

// CreateNodeACL - inserts or updates a node ACL on given network and adds to state
func CreateNodeACL(networkID, nodeID string, defaultVal byte) error {
	if defaultVal != NotAllowed && defaultVal != Allowed {
		defaultVal = NotAllowed
	}

	var commit bool
	dbctx := db.BeginTx(context.TODO())
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	_networkACL := &schema.NetworkACL{
		ID: networkID,
	}
	err := _networkACL.Get(dbctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = _networkACL.Create(dbctx)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	_networkACL.Access.Data()[nodeID] = make(map[string]byte)

	for peerID := range _networkACL.Access.Data() {
		_networkACL.Access.Data()[peerID][nodeID] = defaultVal
		_networkACL.Access.Data()[nodeID][peerID] = defaultVal
	}

	return _networkACL.Update(dbctx)
}

// AreNodesAllowed - checks if nodes are allowed to communicate in their network ACL
func AreNodesAllowed(networkID, node1, node2 string) bool {
	if !servercfg.IsOldAclEnabled() {
		return true
	}

	_networkACL := &schema.NetworkACL{
		ID: networkID,
	}

	err := _networkACL.Get(db.WithContext(context.TODO()))
	if err != nil {
		return false
	}

	_, ok := _networkACL.Access.Data()[node1]
	if !ok {
		return false
	}

	_, ok = _networkACL.Access.Data()[node2]
	if !ok {
		return false
	}

	_, ok = _networkACL.Access.Data()[node1][node2]
	if !ok {
		return false
	}

	_, ok = _networkACL.Access.Data()[node2][node1]
	if !ok {
		return false
	}

	node1Allows := _networkACL.Access.Data()[node1][node2] == Allowed
	node2Allows := _networkACL.Access.Data()[node2][node1] == Allowed

	return node1Allows && node2Allows
}

// ChangeAccess - changes the relationship between two nodes.
func ChangeAccess(networkID, nodeID1, nodeID2 string, value byte) error {
	_networkACL := &schema.NetworkACL{
		ID: networkID,
	}

	var commit bool
	dbctx := db.BeginTx(context.TODO())
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	err := _networkACL.Get(dbctx)
	if err != nil {
		return err
	}

	if _networkACL.Access.Data()[nodeID1] == nil {
		_networkACL.Access.Data()[nodeID1] = make(map[string]byte)
	}

	if _networkACL.Access.Data()[nodeID2] == nil {
		_networkACL.Access.Data()[nodeID2] = make(map[string]byte)
	}

	_networkACL.Access.Data()[nodeID1][nodeID2] = value
	_networkACL.Access.Data()[nodeID2][nodeID1] = value

	err = _networkACL.Update(dbctx)
	if err != nil {
		return err
	}

	commit = true
	return nil
}

// RemoveNodeACL - removes a specific Node's ACL.
func RemoveNodeACL(networkID, nodeID string) error {
	var commit bool
	dbctx := db.BeginTx(context.TODO())
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	_networkACL := &schema.NetworkACL{
		ID: networkID,
	}
	err := _networkACL.Get(dbctx)
	if err != nil {
		return err
	}

	delete(_networkACL.Access.Data(), nodeID)

	for peerID := range _networkACL.Access.Data() {
		delete(_networkACL.Access.Data()[peerID], nodeID)
	}

	return _networkACL.Update(dbctx)
}
