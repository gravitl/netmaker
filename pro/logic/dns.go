package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ValidateNameserverReq(ns *schema.Nameserver) error {
	if ns.Name == "" {
		return errors.New("name is required")
	}
	if ns.NetworkID == "" {
		return errors.New("network is required")
	}
	if len(ns.Servers) == 0 {
		return errors.New("atleast one nameserver should be specified")
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
	if ns.Fallback {
		ns.Domains = []schema.NameserverDomain{}
		ns.MatchAll = false
		return nil
	}
	if !ns.MatchAll && len(ns.Domains) == 0 {
		return errors.New("atleast one match domain is required")
	}
	if !ns.MatchAll {
		for _, domain := range ns.Domains {
			if !logic.IsValidMatchDomain(domain.Domain) {
				return errors.New("invalid match domain")
			}
		}
	}
	return nil
}

func GetNameserversForNode(node *models.Node) (returnNsLi []models.Nameserver) {
	filters := make(map[string]bool)
	if node.Address.IP != nil {
		filters[node.Address.IP.String()] = true
	}

	if node.Address6.IP != nil {
		filters[node.Address6.IP.String()] = true
	}

	ns := &schema.Nameserver{
		NetworkID: node.Network,
	}
	nsLi, _ := ns.ListByNetwork(db.WithContext(context.TODO()))
	for _, nsI := range nsLi {
		if !nsI.Status {
			continue
		}

		filteredIps := logic.FilterOutIPs(nsI.Servers, filters)
		if len(filteredIps) == 0 {
			continue
		}

		_, all := nsI.Tags["*"]
		if all {
			if nsI.Fallback {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:        filteredIps,
					IsFallback: true,
				})
			} else {
				for _, domain := range nsI.Domains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:            filteredIps,
						MatchDomain:    domain.Domain,
						IsSearchDomain: domain.IsSearchDomain,
					})
				}
			}
			continue
		}
		foundTag := false
		for tagI := range node.Tags {
			if _, ok := nsI.Tags[tagI.String()]; ok {
				if nsI.Fallback {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:        filteredIps,
						IsFallback: true,
					})
				} else {
					for _, domain := range nsI.Domains {
						returnNsLi = append(returnNsLi, models.Nameserver{
							IPs:            filteredIps,
							MatchDomain:    domain.Domain,
							IsSearchDomain: domain.IsSearchDomain,
						})
					}
				}
				foundTag = true
			}
			if foundTag {
				break
			}
		}
		if foundTag {
			continue
		}
		if _, ok := nsI.Nodes[node.ID.String()]; ok {
			if nsI.Fallback {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:        filteredIps,
					IsFallback: true,
				})
			} else {
				for _, domain := range nsI.Domains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:            nsI.Servers,
						MatchDomain:    domain.Domain,
						IsSearchDomain: domain.IsSearchDomain,
					})
				}
			}
		}
	}
	return
}

func GetNameserversForHost(h *models.Host) (returnNsLi []models.Nameserver) {
	if h.DNS != "yes" {
		return
	}

	for _, nodeID := range h.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			continue
		}

		filters := make(map[string]bool)
		if node.Address.IP != nil {
			filters[node.Address.IP.String()] = true
		}

		if node.Address6.IP != nil {
			filters[node.Address6.IP.String()] = true
		}

		ns := &schema.Nameserver{
			NetworkID: node.Network,
		}
		nsLi, _ := ns.ListByNetwork(db.WithContext(context.TODO()))
		for _, nsI := range nsLi {
			if !nsI.Status {
				continue
			}

			filteredIps := logic.FilterOutIPs(nsI.Servers, filters)
			if len(filteredIps) == 0 {
				continue
			}

			_, all := nsI.Tags["*"]
			if all {
				if nsI.Fallback {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:        filteredIps,
						IsFallback: true,
					})
				} else {
					for _, domain := range nsI.Domains {
						returnNsLi = append(returnNsLi, models.Nameserver{
							IPs:            filteredIps,
							MatchDomain:    domain.Domain,
							IsSearchDomain: domain.IsSearchDomain,
						})
					}
				}
				continue
			}
			foundTag := false
			for tagI := range node.Tags {
				if _, ok := nsI.Tags[tagI.String()]; ok {
					if nsI.Fallback {
						returnNsLi = append(returnNsLi, models.Nameserver{
							IPs:        filteredIps,
							IsFallback: true,
						})
					} else {
						for _, domain := range nsI.Domains {
							returnNsLi = append(returnNsLi, models.Nameserver{
								IPs:            filteredIps,
								MatchDomain:    domain.Domain,
								IsSearchDomain: domain.IsSearchDomain,
							})
						}
					}
					foundTag = true
				}
				if foundTag {
					break
				}
			}
			if foundTag {
				continue
			}
			if _, ok := nsI.Nodes[node.ID.String()]; ok {
				if nsI.Fallback {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:        filteredIps,
						IsFallback: true,
					})
				} else {
					for _, domain := range nsI.Domains {
						returnNsLi = append(returnNsLi, models.Nameserver{
							IPs:            nsI.Servers,
							MatchDomain:    domain.Domain,
							IsSearchDomain: domain.IsSearchDomain,
						})
					}
				}
			}

		}
	}
	return
}
