package logic

import (
	"encoding/json"
	"os"

	validator "github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
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
	/* if something goes wrong with server DNS, check here
	// commented out bc we were not using IsSplitDNS
	if servercfg.IsSplitDNS() {
		err = SetCorefile(corefilestring)
	}
	*/
	return err
}

// GetDNS - gets the DNS of a current network
func GetDNS(network string) ([]models.DNSEntry, error) {

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
		err = os.MkdirAll(dir+"/config/dnsconfig", 0744)
	}
	if err != nil {
		logger.Log(0, "couldnt find or create /config/dnsconfig")
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

	err = os.WriteFile(dir+"/config/dnsconfig/Corefile", corebytes, 0644)
	if err != nil {
		return err
	}
	return err
}

// GetAllDNS - gets all dns entries
func GetAllDNS() ([]models.DNSEntry, error) {
	var dns []models.DNSEntry
	networks, err := GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.DNSEntry{}, err
	}
	for _, net := range networks {
		netdns, err := GetDNS(net.NetID)
		if err != nil {
			return []models.DNSEntry{}, nil
		}
		dns = append(dns, netdns...)
	}
	return dns, nil
}

// GetDNSEntryNum - gets which entry the dns was
func GetDNSEntryNum(domain string, network string) (int, error) {

	num := 0

	entries, err := GetDNS(network)
	if err != nil {
		return 0, err
	}

	for i := 0; i < len(entries); i++ {

		if domain == entries[i].Name {
			num++
		}
	}

	return num, nil
}

// ValidateDNSCreate - checks if an entry is valid
func ValidateDNSCreate(entry models.DNSEntry) error {

	v := validator.New()

	_ = v.RegisterValidation("name_unique", func(fl validator.FieldLevel) bool {
		num, err := GetDNSEntryNum(entry.Name, entry.Network)
		return err == nil && num == 0
	})

	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		_, err := GetParentNetwork(entry.Network)
		return err == nil
	})

	err := v.Struct(entry)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(1, e.Error())
		}
	}
	return err
}

// ValidateDNSUpdate - validates a DNS update
func ValidateDNSUpdate(change models.DNSEntry, entry models.DNSEntry) error {

	v := validator.New()

	_ = v.RegisterValidation("name_unique", func(fl validator.FieldLevel) bool {
		//if name & net not changing name we are good
		if change.Name == entry.Name && change.Network == entry.Network {
			return true
		}
		num, err := GetDNSEntryNum(change.Name, change.Network)
		return err == nil && num == 0
	})
	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		_, err := GetParentNetwork(change.Network)
		if err != nil {
			logger.Log(0, err.Error())
		}
		return err == nil
	})

	err := v.Struct(change)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(1, e.Error())
		}
	}
	return err
}

// DeleteDNS - deletes a DNS entry
func DeleteDNS(domain string, network string) error {
	key, err := GetRecordKey(domain, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.DNS_TABLE_NAME, key)
	return err
}
