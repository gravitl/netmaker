package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// GetDNS - fetch all DNS entries
func GetDNS() *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, "/api/dns", nil)
}

// GetNodeDNS - fetch all Node DNS entires
func GetNodeDNS(networkName string) *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, fmt.Sprintf("/api/dns/adm/%s/nodes", networkName), nil)
}

// GetCustomDNS - fetch user defined DNS entriees
func GetCustomDNS(networkName string) *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, fmt.Sprintf("/api/dns/adm/%s/custom", networkName), nil)
}

// GetNetworkDNS - fetch DNS entries associated with a network
func GetNetworkDNS(networkName string) *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, "/api/dns/adm/"+networkName, nil)
}

// CreateDNS - create a DNS entry
func CreateDNS(networkName string, payload *models.DNSEntry) *models.DNSEntry {
	return request[models.DNSEntry](http.MethodPost, "/api/dns/"+networkName, payload)
}

// PushDNS - push a DNS entry to CoreDNS
func PushDNS() *string {
	return request[string](http.MethodPost, "/api/dns/adm/pushdns", nil)
}

// DeleteDNS - delete a DNS entry
func DeleteDNS(networkName, domainName string) *string {
	return request[string](http.MethodDelete, fmt.Sprintf("/api/dns/%s/%s", networkName, domainName), nil)
}
