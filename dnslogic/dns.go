package dnslogic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/txn2/txeh"
)

func SetDNS() error {
	hostfile := txeh.Hosts{}
	var corefilestring string
	networks, err := models.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, net := range networks {
		corefilestring = corefilestring + net.NetID + " "
		dns, err := GetDNS(net.NetID)
		if err != nil && !database.IsEmptyRecord(err) {
			return err
		}
		for _, entry := range dns {
			hostfile.AddHost(entry.Address, entry.Name+"."+entry.Network)
		}
	}
	if corefilestring == "" {
		corefilestring = "example.com"
	}

	err = hostfile.SaveAs("./config/dnsconfig/netmaker.hosts")
	if err != nil {
		return err
	}
	if servercfg.IsSplitDNS() {
		err = functions.SetCorefile(corefilestring)
	}
	return err
}

func GetDNS(network string) ([]models.DNSEntry, error) {

	var dns []models.DNSEntry
	dns, err := GetNodeDNS(network)
	if err != nil && !database.IsEmptyRecord(err) {
		return dns, err
	}
	customdns, err := GetCustomDNS(network)
	if err != nil && !database.IsEmptyRecord(err) {
		return dns, err
	}

	dns = append(dns, customdns...)
	return dns, nil
}

func GetNodeDNS(network string) ([]models.DNSEntry, error) {

	var dns []models.DNSEntry

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return dns, err
	}

	for _, value := range collection {
		var entry models.DNSEntry
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			continue
		}
		if err = json.Unmarshal([]byte(value), &entry); node.Network == network && err == nil {
			dns = append(dns, entry)
		}
	}

	return dns, nil
}

func GetCustomDNS(network string) ([]models.DNSEntry, error) {

	var dns []models.DNSEntry

	collection, err := database.FetchRecords(database.DNS_TABLE_NAME)
	if err != nil {
		return dns, err
	}
	for _, value := range collection { // filter for entries based on network
		var entry models.DNSEntry
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			continue
		}

		if entry.Network == network {
			dns = append(dns, entry)
		}
	}

	return dns, err
}
