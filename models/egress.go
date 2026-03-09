package models

import "github.com/gravitl/netmaker/schema"

type EgressReq struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Network     string               `json:"network"`
	Description string               `json:"description"`
	Nodes       map[string]int       `json:"nodes"`
	Tags        map[string]int       `json:"tags"`
	Range       string               `json:"range"`
	Domain      string               `json:"domain"`
	Nat         bool                 `json:"nat"`
	Mode        schema.EgressNATMode `json:"mode"`
	Status      bool                 `json:"status"`
	IsInetGw    bool                 `json:"is_internet_gateway"`
}
