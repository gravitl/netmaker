// TODO:  Either add a returnNetwork and returnKey, or delete this
package models

import "github.com/gravitl/netmaker/schema"

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

func (action DNSUpdateAction) String() string {
	return [...]string{"DNSDeleteByIP", "DNSDeletByName", "DNSReplaceName", "DNSReplaceIP", "DNSInsert"}[action]
}

// DNSError.Error implementation of error interface
func (e DNSError) Error() string {
	return "error publishing dns update"
}

// DNSError error struct capable of holding multiple error messages
type DNSError struct {
	ErrorStrings []string
}

// DNSUpdate data for updating entries in /etc/hosts
type DNSUpdate struct {
	Action     DNSUpdateAction
	Name       string
	NewName    string
	Address    string
	NewAddress string
}

type DNSEntryType = schema.DNSEntryType

const (
	DNSEntryType_Node   = schema.DNSEntryType_Node
	DNSEntryType_Custom = schema.DNSEntryType_Custom
)

type DNSEntry = schema.DNSEntry

type NameserverReq struct {
	Name        string   `json:"name"`
	Network     string   `json:"network"`
	Description string   ` json:"description"`
	Servers     []string `json:"servers"`
	MatchDomain string   `json:"match_domain"`
	Tags        []string `json:"tags"`
	Status      bool     `gorm:"status" json:"status"`
}
