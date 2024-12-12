package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"

	validator "github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/txn2/txeh"
)

// SetDNS - sets the dns on file
func SetDNS() error {
	hostfile, err := txeh.NewHosts(&txeh.HostsConfig{})
	if err != nil {
		return err
	}
	var corefilestring string
	networks, err := GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, net := range networks {
		corefilestring = corefilestring + net.NetID + " "
		dns, err := GetDNS(models.NetworkID(net.NetID))
		if err != nil && !database.IsEmptyRecord(err) {
			return err
		}
		for _, entry := range dns {
			hostfile.AddHost(entry.Address, entry.Name)
		}
	}
	dns := GetExtclientDNS()
	for _, entry := range dns {
		hostfile.AddHost(entry.Address, entry.Name)
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
func GetDNS(networkID models.NetworkID) ([]models.DNSEntry, error) {

	dns, err := GetNodeDNS(networkID)
	if err != nil && !database.IsEmptyRecord(err) {
		return dns, err
	}
	customdns, err := GetCustomDNS(networkID.String())
	if err != nil && !database.IsEmptyRecord(err) {
		return dns, err
	}

	dns = append(dns, customdns...)
	return dns, nil
}

// GetExtclientDNS - gets all extclients dns entries
func GetExtclientDNS() []models.DNSEntry {
	extclients, err := GetAllExtClients()
	if err != nil {
		return []models.DNSEntry{}
	}
	var dns []models.DNSEntry
	for _, extclient := range extclients {
		var entry = models.DNSEntry{}
		entry.Name = fmt.Sprintf("%s.%s", extclient.ClientID, extclient.Network)
		entry.Network = extclient.Network
		if extclient.Address != "" {
			entry.Address = extclient.Address
		}
		if extclient.Address6 != "" {
			entry.Address6 = extclient.Address6
		}
		dns = append(dns, entry)
	}
	return dns
}

// GetNodeDNS - gets the DNS of a network node
func GetNodeDNS(networkID models.NetworkID) ([]models.DNSEntry, error) {

	var dns []models.DNSEntry
	net, err := GetNetwork(networkID.String())
	if err != nil {
		return []models.DNSEntry{}, err
	}
	nodes, err := GetNetworkNodes(networkID.String())
	if err != nil {
		return dns, err
	}
	defaultDomain := servercfg.GetDefaultDomain()
	for _, node := range nodes {
		if node.Network != networkID.String() {
			continue
		}
		host, err := GetHost(node.HostID.String())
		if err != nil {
			continue
		}
		var entry = models.DNSEntry{}
		entry.Name = fmt.Sprintf("%s.%s.%s", host.Name, net.Name, defaultDomain)
		entry.Network = net.NetID
		if node.Address.IP != nil {
			entry.Address = node.Address.IP.String()
		}
		if node.Address6.IP != nil {
			entry.Address6 = node.Address6.IP.String()
		}
		dns = append(dns, entry)
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

	err = os.MkdirAll(dir+"/config/dnsconfig", 0744)
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
	err = os.WriteFile(dir+"/config/dnsconfig/Corefile", []byte(corefile), 0644)
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
		netdns, err := GetDNS(models.NetworkID(net.NetID))
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

	entries, err := GetDNS(models.NetworkID(network))
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

// SortDNSEntrys - Sorts slice of DNSEnteys by their Address alphabetically with numbers first
func SortDNSEntrys(unsortedDNSEntrys []models.DNSEntry) {
	sort.Slice(unsortedDNSEntrys, func(i, j int) bool {
		return unsortedDNSEntrys[i].Address < unsortedDNSEntrys[j].Address
	})
}

// IsNetworkNameValid - checks if a netid of a network uses valid characters
func IsDNSEntryValid(d string) bool {
	re := regexp.MustCompile(`^[A-Za-z0-9-.]+$`)
	return re.MatchString(d)
}

// ValidateDNSCreate - checks if an entry is valid
func ValidateDNSCreate(entry models.DNSEntry) error {
	if !IsDNSEntryValid(entry.Name) {
		return errors.New("invalid input. Only uppercase letters (A-Z), lowercase letters (a-z), numbers (0-9), minus sign (-) and dots (.) are allowed")
	}
	v := validator.New()

	_ = v.RegisterValidation("whitespace", func(f1 validator.FieldLevel) bool {
		match, _ := regexp.MatchString(`\s`, entry.Name)
		return !match
	})

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

	_ = v.RegisterValidation("whitespace", func(f1 validator.FieldLevel) bool {
		match, _ := regexp.MatchString(`\s`, entry.Name)
		return !match
	})

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

// CreateDNS - creates a DNS entry
func CreateDNS(entry models.DNSEntry) (models.DNSEntry, error) {

	k, err := GetRecordKey(entry.Name, entry.Network)
	if err != nil {
		return models.DNSEntry{}, err
	}

	data, err := json.Marshal(&entry)
	if err != nil {
		return models.DNSEntry{}, err
	}

	err = database.Insert(k, string(data), database.DNS_TABLE_NAME)
	return entry, err
}
