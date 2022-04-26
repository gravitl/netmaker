//TODO:  Either add a returnNetwork and returnKey, or delete this
package models

// DNSEntry - a DNS entry represented as struct
type DNSEntry struct {
	Address  string `json:"address" bson:"address" validate:"ip"`
	Address6 string `json:"address6" bson:"address6"`
	Name     string `json:"name" bson:"name" validate:"required,name_unique,min=1,max=192"`
	Network  string `json:"network" bson:"network" validate:"network_exists"`
}
