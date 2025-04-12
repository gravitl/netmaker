package models

import "net"

type Egress struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	EgressNode  string    `json:"egress_node"`
	EgressGroup []TagID   `json:"egress_group"`
	Range       net.IPNet `json:"range"`
}
