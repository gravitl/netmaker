package models

import "net"

type Egress struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	RoutingNode  string    `json:"routing_node"`
	RoutingGroup []TagID   `json:"routing_tags"`
	Range        net.IPNet `json:"range"`
}
