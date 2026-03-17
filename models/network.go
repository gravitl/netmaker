package models

import (
	"time"
)

// Network Struct - contains info for a given unique network
// At  some point, need to replace all instances of Name with something else like  Identifier
type Network struct {
	AddressRange        string   `json:"addressrange" bson:"addressrange" validate:"omitempty,cidrv4"`
	AddressRange6       string   `json:"addressrange6" bson:"addressrange6" validate:"omitempty,cidrv6"`
	NetID               string   `json:"netid" bson:"netid" validate:"required,min=1,max=32,netid_valid"`
	NodesLastModified   int64    `json:"nodeslastmodified" bson:"nodeslastmodified" swaggertype:"primitive,integer" format:"int64"`
	NetworkLastModified int64    `json:"networklastmodified" bson:"networklastmodified" swaggertype:"primitive,integer" format:"int64"`
	DefaultKeepalive    int32    `json:"defaultkeepalive" bson:"defaultkeepalive" validate:"omitempty,max=1000"`
	IsIPv4              string   `json:"isipv4" bson:"isipv4" validate:"checkyesorno"`
	IsIPv6              string   `json:"isipv6" bson:"isipv6" validate:"checkyesorno"`
	DefaultMTU          int32    `json:"defaultmtu" bson:"defaultmtu"`
	DefaultACL          string   `json:"defaultacl" bson:"defaultacl" yaml:"defaultacl" validate:"checkyesorno"`
	NameServers         []string `json:"dns_nameservers"`
	AutoJoin            string   `json:"auto_join"`
	AutoRemove          string   `json:"auto_remove"`
	AutoRemoveTags      []string `json:"auto_remove_tags"`
	AutoRemoveThreshold int      `json:"auto_remove_threshold_mins"`
	JITEnabled          string   `json:"jit_enabled" bson:"jit_enabled" validate:"checkyesorno"`
	// VirtualNATPoolIPv4 is the IPv4 CIDR pool from which virtual NAT ranges are allocated for egress gateways
	VirtualNATPoolIPv4 string `json:"virtual_nat_pool_ipv4"`
	// VirtualNATSitePrefixLenIPv4 is the prefix length (e.g., 24) for individual site allocations from the IPv4 virtual NAT pool
	VirtualNATSitePrefixLenIPv4 int    `json:"virtual_nat_site_prefixlen_ipv4"`
	CreatedBy                   string `json:"created_by"`
	CreatedAt                   time.Time
}

// SaveData - sensitive fields of a network that should be kept the same
type SaveData struct { // put sensitive fields here
	NetID string `json:"netid" bson:"netid" validate:"required,min=1,max=32,netid_valid"`
}
