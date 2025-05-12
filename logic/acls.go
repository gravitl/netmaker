package logic

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

var GetDefaultPolicy = func(netID models.NetworkID, ruleType models.AclPolicyType) (models.Acl, error) {

	return models.Acl{}, nil
}

var CleanUpEgressPolicies = func(e *schema.Egress) {}
