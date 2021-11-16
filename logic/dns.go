package logic

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/txn2/txeh"
)

// SetDNS - sets the dns on file
func SetDNS() error {
	hostfile := txeh.Hosts{}
	var corefilestring string
	networks, err := GetNetworks()
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
		err = SetCorefile(corefilestring)
	}
	return err
}

// GetDNS - gets the DNS of a current network
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

// GetNodeDNS - gets the DNS of a network node
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

// GetCustomDNS - gets the custom DNS of a network
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

// SetCorefile - sets the core file of the system
func SetCorefile(domains string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	_, err = os.Stat(dir + "/config/dnsconfig")
	if os.IsNotExist(err) {
		os.Mkdir(dir+"/config/dnsconfig", 744)
	} else if err != nil {
		Log("couldnt find or create /config/dnsconfig", 0)
		return err
	}

	corefile := domains + ` {
    reload 15s
    hosts /root/dnsconfig/netmaker.hosts {
	fallthrough	
    }
    forward . 8.8.8.8 8.8.4.4
    log
}
`
	corebytes := []byte(corefile)

	err = ioutil.WriteFile(dir+"/config/dnsconfig/Corefile", corebytes, 0644)
	if err != nil {
		return err
	}
	return err
}
