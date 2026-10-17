package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ValidateNameserverReq(ctx context.Context, ns *schema.Nameserver) error {
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
			_, err := GetTag(ctx, models.TagID(tagI))
			if err != nil {
				return errors.New("invalid tag")
			}
		}
	}
	if _, ok := ns.Users["*"]; ok && len(ns.Users) > 1 {
		return errors.New("all users (*) cannot be combined with specific users")
	}
	for username := range ns.Users {
		if username == "*" {
			continue
		}
		user := &schema.User{Username: username}
		if err := user.GetWithMembership(ctx); err != nil {
			return errors.New("invalid user " + username)
		}
	}
	for groupID := range ns.UserGroups {
		if _, err := GetUserGroup(ctx, schema.UserGroupID(groupID)); err != nil {
			return errors.New("invalid user group " + groupID)
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

func GetNameserversForNode(ctx context.Context, node *models.Node) (returnNsLi []models.Nameserver) {
	return getNameserversForNode(ctx, node, logic.NodeOwnerUsername(node))
}

func GetNameserversForHost(ctx context.Context, h *schema.Host) (returnNsLi []models.Nameserver) {
	if h.DNS != "yes" {
		return
	}

	for _, nodeID := range h.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		owner := logic.NodeOwnerUsername(&node)
		if owner == "" {
			owner = h.OwnerUsername
		}
		returnNsLi = append(returnNsLi, getNameserversForNode(ctx, &node, owner)...)
	}
	return
}

// getNameserversForNode returns the nameservers in the node's network that
// target the node, either via all ("*"), the node's tags, the owning user or
// one of the owner's user groups, or the node itself.
func getNameserversForNode(ctx context.Context, node *models.Node, owner string) (returnNsLi []models.Nameserver) {
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
	nsLi, _ := ns.ListByNetwork(ctx)

	var ownerGroups map[schema.UserGroupID]struct{}
	if owner != "" {
		user := &schema.User{Username: owner}
		if err := user.GetWithMembership(ctx); err == nil {
			ownerGroups = user.UserGroups.Data()
		}
	}

	for _, nsI := range nsLi {
		if !nsI.Status {
			continue
		}

		filteredIps := logic.FilterOutIPs(nsI.Servers, filters)
		if len(filteredIps) == 0 {
			continue
		}

		if !nameserverTargetsNode(&nsI, node, owner, ownerGroups) {
			continue
		}

		if nsI.Fallback {
			returnNsLi = append(returnNsLi, models.Nameserver{
				IPs:        filteredIps,
				IsFallback: true,
			})
			continue
		}
		for _, domain := range nsI.Domains {
			returnNsLi = append(returnNsLi, models.Nameserver{
				IPs:            filteredIps,
				MatchDomain:    domain.Domain,
				IsSearchDomain: domain.IsSearchDomain,
				IsADDomain:     domain.IsADDomain,
			})
		}
	}
	return
}

// nameserverTargetsNode reports whether ns applies to node, either via all
// resources ("*" tag, nodes only), one of the node's tags, the owning user (or
// all users via "*", user devices only), one of the owner's user groups, or
// the node itself.
func nameserverTargetsNode(ns *schema.Nameserver, node *models.Node, owner string, ownerGroups map[schema.UserGroupID]struct{}) bool {
	if logic.NameserverTargetsAll(ns, owner) {
		return true
	}
	for tagI := range node.Tags {
		if _, ok := ns.Tags[tagI.String()]; ok {
			return true
		}
	}
	if owner != "" {
		if _, ok := ns.Users[owner]; ok {
			return true
		}
		for groupID := range ownerGroups {
			if _, ok := ns.UserGroups[groupID.String()]; ok {
				return true
			}
		}
	}
	_, ok := ns.Nodes[node.ID.String()]
	return ok
}

func RemoveTagFromNameservers(tagID models.TagID, netID schema.NetworkID) error {
	nameservers, err := (&schema.Nameserver{
		NetworkID: netID.String(),
	}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	var multiErr error
	for _, nameserver := range nameservers {
		delete(nameserver.Tags, tagID.String())
		err := nameserver.Update(db.WithContext(context.TODO()))
		if err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}

	return multiErr
}
