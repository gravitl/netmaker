package logic

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func GetCurrentServerUsage() (limits models.Usage) {
	limits.SetDefaults()
	hosts, hErr := GetAllHostsWithStatus(models.OnlineSt)
	if hErr == nil {
		limits.Hosts = len(hosts)
	}
	clients, cErr := GetAllExtClientsWithStatus(models.OnlineSt)
	if cErr == nil {
		limits.Clients = len(clients)
	}
	limits.Users, _ = (&schema.User{}).Count(db.WithContext(context.TODO()))
	limits.Networks, _ = (&schema.Network{}).Count(db.WithContext(context.TODO()))
	limits.Egresses, _ = (&schema.Egress{}).Count(db.WithContext(context.TODO()))

	nodes, _ := GetAllNodes()

	for _, client := range clients {
		nodes = append(nodes, client.ConvertToStaticNode())
	}

	limits.NetworkUsage = make(map[string]models.NetworkUsage)
	networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
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
