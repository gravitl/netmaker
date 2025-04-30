package converters

import (
	"github.com/gravitl/netmaker/logic/nodeacls"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func ToSchemaNetworkACL(networkID string, aclContainer nodeacls.ACLContainer) schema.NetworkACL {
	_networkACL := schema.NetworkACL{
		ID:     networkID,
		Access: datatypes.JSONType[map[string]map[string]byte]{},
	}

	for nodeID := range aclContainer {
		_networkACL.Access.Data()[string(nodeID)] = make(map[string]byte)

		for peerID := range aclContainer[nodeID] {
			_networkACL.Access.Data()[string(nodeID)][string(peerID)] = aclContainer[nodeID][peerID]
		}
	}

	return _networkACL
}

func ToACLContainer(_networkACL schema.NetworkACL) nodeacls.ACLContainer {
	var aclContainer = nodeacls.ACLContainer{}

	for nodeID := range _networkACL.Access.Data() {
		aclContainer[nodeacls.AclID(nodeID)] = make(nodeacls.ACL)

		for peerID := range _networkACL.Access.Data()[nodeID] {
			aclContainer[nodeacls.AclID(nodeID)][nodeacls.AclID(peerID)] = _networkACL.Access.Data()[nodeID][peerID]
		}
	}

	return aclContainer
}
