package logic

import "github.com/gravitl/netmaker/models"

func ValidateEgressReq(e *models.Egress) bool {
	if e.Network == "" {
		return false
	}
	if e.Range.IP == nil {
		return false
	}
	if len(e.Nodes) != 0 {
		for _, nodeID := range e.Nodes {
			_, err := GetNodeByID(nodeID)
			if err != nil {
				return false
			}
		}
	}
	return true
}
