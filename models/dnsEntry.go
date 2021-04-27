//TODO:  Either add a returnNetwork and returnKey, or delete this
package models

type DNSEntry struct {
	Address	string `json:"address" bson:"address" validate:"address_valid`
	Name	string `json:"name" bson:"name" validate:"omitempty,name_valid,name_unique,max=120"`
	Network	string `json:"network" bson:"network" validate:"network_exists"`
}
