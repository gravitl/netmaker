package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

func GetDNS() *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, "/api/dns", nil)
}

func GetNodeDNS(networkName string) *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, fmt.Sprintf("/api/dns/adm/%s/nodes", networkName), nil)
}

func GetCustomDNS(networkName string) *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, fmt.Sprintf("/api/dns/adm/%s/custom", networkName), nil)
}

func GetNetworkDNS(networkName string) *[]models.DNSEntry {
	return request[[]models.DNSEntry](http.MethodGet, "/api/dns/adm/"+networkName, nil)
}

func CreateDNS(networkName string, payload *models.DNSEntry) *models.DNSEntry {
	return request[models.DNSEntry](http.MethodPost, "/api/dns/"+networkName, payload)
}
