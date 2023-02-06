// TODO:  Either add a returnNetwork and returnKey, or delete this
package models

// DNSUpdateAction identifies the action to be performed with the dns update data
type DNSUpdateAction int

const (
	// DNSDeleteByIP delete the dns entry
	DNSDeleteByIP = iota
	// DNSDeleteByName delete the dns entry
	DNSDeleteByName
	// DNSReplaceName replace the dns entry
	DNSReplaceName
	// DNSReplaceIP resplace the dns entry
	DNSReplaceIP
	// DNSInsert insert a new dns entry
	DNSInsert
)

// DNSUpdate data for updating entries in /etc/hosts
type DNSUpdate struct {
	Action  DNSUpdateAction
	Name    string
	NewName string
	Address string
}

// DNSEntry - a DNS entry represented as struct
type DNSEntry struct {
	Address  string `json:"address" bson:"address" validate:"ip"`
	Address6 string `json:"address6" bson:"address6"`
	Name     string `json:"name" bson:"name" validate:"required,name_unique,min=1,max=192"`
	Network  string `json:"network" bson:"network" validate:"network_exists"`
}
