//TODO:  Either add a returnNetwork and returnKey, or delete this
package models

type DNSEntry struct {
	Address	string `json:"address" bson:"address"`
	Name	string `json:"name" bson:"name"`
	Network	string `json:"network" bson:"network"`
}
