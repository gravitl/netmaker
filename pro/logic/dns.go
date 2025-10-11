package logic

import (
	"context"
	"errors"
	"net"

	"github.com/gravitl/netmaker/db"
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
	network, err := logic.GetNetwork(ns.NetworkID)
	if err != nil {
		return errors.New("invalid network id")
	}
	_, cidr, err4 := net.ParseCIDR(network.AddressRange)
	_, cidr6, err6 := net.ParseCIDR(network.AddressRange6)
	for _, nsIPStr := range ns.Servers {
		nsIP := net.ParseIP(nsIPStr)
		if nsIP == nil {
			return errors.New("invalid nameserver " + nsIPStr)
		}
		if err4 == nil && nsIP.To4() != nil {
			if cidr.Contains(nsIP) {
				return errors.New("cannot use netmaker IP as nameserver")
			}
		} else if err6 == nil && cidr6.Contains(nsIP) {
			return errors.New("cannot use netmaker IP as nameserver")
		}
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

func GetNameserversForNode(node *models.Node) (returnNsLi []models.Nameserver) {
	ns := &schema.Nameserver{
		NetworkID: node.Network,
	}
	nsLi, _ := ns.ListByNetwork(db.WithContext(context.TODO()))
	for _, nsI := range nsLi {
		if !nsI.Status {
			continue
		}
		_, all := nsI.Tags["*"]
		if all {
			for _, matchDomain := range nsI.MatchDomains {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:         nsI.Servers,
					MatchDomain: matchDomain,
				})
			}
			continue
		}
		foundTag := false
		for tagI := range node.Tags {
			if _, ok := nsI.Tags[tagI.String()]; ok {
				for _, matchDomain := range nsI.MatchDomains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:         nsI.Servers,
						MatchDomain: matchDomain,
					})
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
			for _, matchDomain := range nsI.MatchDomains {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:         nsI.Servers,
					MatchDomain: matchDomain,
				})
			}
		}
	}
	if node.IsInternetGateway {
		globalNs := models.Nameserver{
			MatchDomain: ".",
		}
		for _, nsI := range logic.GlobalNsList {
			globalNs.IPs = append(globalNs.IPs, nsI.IPs...)
		}
		returnNsLi = append(returnNsLi, globalNs)
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
		ns := &schema.Nameserver{
			NetworkID: node.Network,
		}
		nsLi, _ := ns.ListByNetwork(db.WithContext(context.TODO()))
		for _, nsI := range nsLi {
			if !nsI.Status {
				continue
			}
			_, all := nsI.Tags["*"]
			if all {
				for _, matchDomain := range nsI.MatchDomains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:         nsI.Servers,
						MatchDomain: matchDomain,
					})
				}
				continue
			}
			foundTag := false
			for tagI := range node.Tags {
				if _, ok := nsI.Tags[tagI.String()]; ok {
					for _, matchDomain := range nsI.MatchDomains {
						returnNsLi = append(returnNsLi, models.Nameserver{
							IPs:         nsI.Servers,
							MatchDomain: matchDomain,
						})
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
				for _, matchDomain := range nsI.MatchDomains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:         nsI.Servers,
						MatchDomain: matchDomain,
					})
				}
			}

		}
		if node.IsInternetGateway {
			globalNs := models.Nameserver{
				MatchDomain: ".",
			}
			for _, nsI := range logic.GlobalNsList {
				globalNs.IPs = append(globalNs.IPs, nsI.IPs...)
			}
			returnNsLi = append(returnNsLi, globalNs)
		}
	}
	return
}
