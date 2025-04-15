package logic

import (
	"net"

	"github.com/gravitl/netmaker/models"
)

func ValidateEgressReq(e *models.Egress) bool {
	if e.Network == "" {
		return false
	}
	_, err := GetNetwork(e.Network)
	if err != nil {
		return false
	}
	if e.Range == "" {
		return false
	}
	_, _, err = net.ParseCIDR(e.Range)
	if err != nil {
		return false
	}
	err = ValidateEgressRange(e.Network, []string{e.Range})
	if err != nil {
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
