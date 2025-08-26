package logic

import (
	"errors"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ValidateNameserverReq(ns schema.Nameserver) error {
	if ns.Name == "" {
		return errors.New("name is required")
	}
	if ns.NetworkID == "" {
		return errors.New("network is required")
	}
	if len(ns.Servers) == 0 {
		return errors.New("atleast one nameserver should be specified")
	}
	if !ns.MatchAll && len(ns.MatchDomains) == 0 {
		return errors.New("atleast one match domain is required")
	}
	if !ns.MatchAll {
		for _, matchDomain := range ns.MatchDomains {
			if !logic.IsValidMatchDomain(matchDomain) {
				return errors.New("invalid match domain")
			}
		}
	}
	if len(ns.Tags) > 0 {
		for tagI := range ns.Tags {
			if tagI == "*" {
				continue
			}
			_, err := GetTag(models.TagID(tagI))
			if err != nil {
				return errors.New("invalid tag")
			}
		}
	}
	return nil
}
