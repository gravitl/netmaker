package logic

import "github.com/gravitl/netmaker/models"

func ValidateEgressReq(e *models.Egress) bool {
	if e.Network == "" {
		return false
	}
	if e.Range == "" {
		return false
	}
	if len(e.Nodes) != 0 {
		for k := range e.Nodes {
			_, err := GetNodeByID(k)
			if err != nil {
				return false
			}
		}
	}
	return true
}

// func GetEgressFwRules(targetNode *models.Node) (m map[string]models.EgressInfo) {
// 	eli, _ := (&models.Egress{}).ListByNetwork()

// }
