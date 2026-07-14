package logic

import (
	"context"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func GetCurrentServerUsage(ctx context.Context) (limits models.Usage) {
	limits.SetDefaults()
	hosts, hErr := GetAllHostsWithStatus(ctx, schema.OnlineSt)
	if hErr == nil {
		limits.Hosts = len(hosts)
	}
	clients, cErr := GetAllExtClientsWithStatus(ctx, schema.OnlineSt)
	if cErr == nil {
		limits.Clients = len(clients)
	}
	limits.Users, _ = (&schema.User{}).Count(ctx)
	limits.Networks, _ = (&schema.Network{}).Count(ctx)
	limits.Egresses, _ = (&schema.Egress{}).Count(ctx)

	nodes, _ := GetAllNodes(ctx)

	for _, client := range clients {
		nodes = append(nodes, models.ConvertToStaticNode(client))
	}

	limits.NetworkUsage = make(map[string]models.NetworkUsage)
	networks, _ := (&schema.Network{}).ListAll(ctx)
	for _, network := range networks {
		limits.NetworkUsage[network.Name] = models.NetworkUsage{}
	}

	for _, node := range nodes {
		netUsage, ok := limits.NetworkUsage[node.Network]
		if !ok {
			// if network doesn't exist, this node is probably awaiting cleanup.
			// so ignore.
			continue
		}

		netUsage.Nodes++
		if node.IsStatic {
			netUsage.Clients++
		}
		if node.IsIngressGateway {
			limits.Ingresses++
			netUsage.Ingresses++
		}
		if node.EgressDetails.IsEgressGateway {
			netUsage.Egresses++
		}
		if node.IsRelay {
			limits.Relays++
			netUsage.Relays++
		}
		if node.IsInternetGateway {
			limits.InternetGateways++
			netUsage.InternetGateways++
		}
		if node.IsAutoRelay {
			limits.FailOvers++
			netUsage.FailOvers++
		}

		limits.NetworkUsage[node.Network] = netUsage
	}

	return
}
