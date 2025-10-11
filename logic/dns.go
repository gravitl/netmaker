package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"

	validator "github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/txn2/txeh"
)

var GetNameserversForNode = getNameserversForNode
var GetNameserversForHost = getNameserversForHost
var ValidateNameserverReq = validateNameserverReq

type GlobalNs struct {
	ID  string   `json:"id"`
	IPs []string `json:"ips"`
}

var GlobalNsList = map[string]GlobalNs{
	"Google": {
		ID: "Google",
		IPs: []string{
			"8.8.8.8",
			"8.8.4.4",
			"2001:4860:4860::8888",
			"2001:4860:4860::8844",
		},
	},
	"Cloudflare": {
		ID: "Cloudflare",
		IPs: []string{
			"1.1.1.1",
			"1.0.0.1",
			"2606:4700:4700::1111",
			"2606:4700:4700::1001",
		},
	},
	"Quad9": {
		ID: "Quad9",
		IPs: []string{
			"9.9.9.9",
			"149.112.112.112",
			"2620:fe::fe",
			"2620:fe::9",
		},
	},
}

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
		dns, err := GetDNS(net.NetID)
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

func EgressDNs(network string) (entries []models.DNSEntry) {
	egs, _ := (&schema.Egress{
		Network: network,
	}).ListByNetwork(db.WithContext(context.TODO()))
	for _, egI := range egs {
		if egI.Domain != "" && len(egI.DomainAns) > 0 {
			entry := models.DNSEntry{
				Name: egI.Domain,
			}
			for _, domainAns := range egI.DomainAns {
				ip, _, err := net.ParseCIDR(domainAns)
				if err == nil {
					if ip.To4() != nil {
						entry.Address = ip.String()
					} else {
						entry.Address6 = ip.String()
					}
				}
			}
			entries = append(entries, entry)
		}
	}
	return
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
func GetNodeDNS(network string) ([]models.DNSEntry, error) {

	var dns []models.DNSEntry

	nodes, err := GetNetworkNodes(network)
	if err != nil {
		return dns, err
	}
	defaultDomain := GetDefaultDomain()
	for _, node := range nodes {
		if node.Network != network {
			continue
		}
		host, err := GetHost(node.HostID.String())
		if err != nil {
			continue
		}
		var entry = models.DNSEntry{}
		if defaultDomain == "" {
			entry.Name = fmt.Sprintf("%s.%s", host.Name, network)
		} else {
			entry.Name = fmt.Sprintf("%s.%s.%s", host.Name, network, defaultDomain)
		}
		entry.Network = network
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

func GetGwDNS(node *models.Node) string {
	if !servercfg.GetManageDNS() {
		return ""
	}
	h, err := GetHost(node.HostID.String())
	if err != nil {
		return ""
	}
	if h.DNS != "yes" {
		return ""
	}
	dns := []string{}
	if node.Address.IP != nil {
		dns = append(dns, node.Address.IP.String())
	}
	if node.Address6.IP != nil {
		dns = append(dns, node.Address6.IP.String())
	}
	return strings.Join(dns, ",")

}

func SetDNSOnWgConfig(gwNode *models.Node, extclient *models.ExtClient) {
	extclient.DNS = GetGwDNS(gwNode)
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

func validateNameserverReq(ns schema.Nameserver) error {
	if ns.Name == "" {
		return errors.New("name is required")
	}
	if ns.NetworkID == "" {
		return errors.New("network is required")
	}
	if len(ns.Servers) == 0 {
		return errors.New("atleast one nameserver should be specified")
	}
	network, err := GetNetwork(ns.NetworkID)
	if err != nil {
		return errors.New("invalid network id")
	}
	_, cidr, err4 := net.ParseCIDR(network.AddressRange)
	_, cidr6, err6 := net.ParseCIDR(network.AddressRange6)
	for _, nsIPStr := range ns.Servers {
		nsIP := net.ParseIP(nsIPStr)
		if nsIP == nil {
			return errors.New("invalid nameserver " + nsIPStr)
		}
		if err4 == nil && nsIP.To4() != nil {
			if cidr.Contains(nsIP) {
				return errors.New("cannot use netmaker IP as nameserver")
			}
		} else if err6 == nil && cidr6.Contains(nsIP) {
			return errors.New("cannot use netmaker IP as nameserver")
		}
	}
	if !ns.MatchAll && len(ns.MatchDomains) == 0 {
		return errors.New("atleast one match domain is required")
	}
	if !ns.MatchAll {
		for _, matchDomain := range ns.MatchDomains {
			if !IsValidMatchDomain(matchDomain) {
				return errors.New("invalid match domain")
			}
		}
	}
	// check if valid broadcast peers are added
	if len(ns.Nodes) > 0 {
		for nodeID := range ns.Nodes {
			node, err := GetNodeByID(nodeID)
			if err != nil {
				return errors.New("invalid node")
			}
			if node.Network != ns.NetworkID {
				return errors.New("invalid network node")
			}
		}
	}

	return nil
}

func getNameserversForNode(node *models.Node) (returnNsLi []models.Nameserver) {
	ns := &schema.Nameserver{
		NetworkID: node.Network,
	}
	nsLi, _ := ns.ListByNetwork(db.WithContext(context.TODO()))
	for _, nsI := range nsLi {
		if !nsI.Status {
			continue
		}
		_, all := nsI.Tags["*"]
		if all {
			for _, matchDomain := range nsI.MatchDomains {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:         nsI.Servers,
					MatchDomain: matchDomain,
				})
			}
			continue
		}

		if _, ok := nsI.Nodes[node.ID.String()]; ok {
			for _, matchDomain := range nsI.MatchDomains {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:         nsI.Servers,
					MatchDomain: matchDomain,
				})
			}
		}

	}
	if node.IsInternetGateway {
		globalNs := models.Nameserver{
			MatchDomain: ".",
		}
		for _, nsI := range GlobalNsList {
			globalNs.IPs = append(globalNs.IPs, nsI.IPs...)
		}
		returnNsLi = append(returnNsLi, globalNs)
	}
	return
}

func getNameserversForHost(h *models.Host) (returnNsLi []models.Nameserver) {
	if h.DNS != "yes" {
		return
	}
	for _, nodeID := range h.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		ns := &schema.Nameserver{
			NetworkID: node.Network,
		}
		nsLi, _ := ns.ListByNetwork(db.WithContext(context.TODO()))
		for _, nsI := range nsLi {
			if !nsI.Status {
				continue
			}
			_, all := nsI.Tags["*"]
			if all {
				for _, matchDomain := range nsI.MatchDomains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:         nsI.Servers,
						MatchDomain: matchDomain,
					})
				}
				continue
			}

			if _, ok := nsI.Nodes[node.ID.String()]; ok {
				for _, matchDomain := range nsI.MatchDomains {
					returnNsLi = append(returnNsLi, models.Nameserver{
						IPs:         nsI.Servers,
						MatchDomain: matchDomain,
					})
				}

			}

		}
		if node.IsInternetGateway {
			globalNs := models.Nameserver{
				MatchDomain: ".",
			}
			for _, nsI := range GlobalNsList {
				globalNs.IPs = append(globalNs.IPs, nsI.IPs...)
			}
			returnNsLi = append(returnNsLi, globalNs)
		}
	}
	return
}

// IsValidMatchDomain reports whether s is a valid "match domain".
// Rules (simple/ASCII):
//   - "~." is allowed (match all).
//   - Optional leading "~" allowed (e.g., "~example.com").
//   - Optional single trailing "." allowed (FQDN form).
//   - No wildcards "*", no leading ".", no underscores.
//   - Labels: letters/digits/hyphen (LDH), 1–63 chars, no leading/trailing hyphen.
//   - Total length (without trailing dot) ≤ 253.
func IsValidMatchDomain(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if s == "~." { // special case: match-all
		return true
	}

	// Strip optional leading "~"
	if strings.HasPrefix(s, "~") {
		s = s[1:]
		if s == "" {
			return false
		}
	}

	// Allow exactly one trailing dot
	if strings.HasSuffix(s, ".") {
		s = s[:len(s)-1]
		if s == "" {
			return false
		}
	}

	// Disallow leading dot, wildcards, underscores
	if strings.HasPrefix(s, ".") || strings.Contains(s, "*") || strings.Contains(s, "_") {
		return false
	}

	// Lowercase for ASCII checks
	s = strings.ToLower(s)

	// Length check
	if len(s) > 253 {
		return false
	}

	// Label regex: LDH, 1–63, no leading/trailing hyphen
	reLabel := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

	parts := strings.Split(s, ".")
	for _, lbl := range parts {
		if len(lbl) == 0 || len(lbl) > 63 {
			return false
		}
		if !reLabel.MatchString(lbl) {
			return false
		}
	}
	return true
}
