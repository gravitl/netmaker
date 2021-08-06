package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/txn2/txeh"
)

func dnsHandlers(r *mux.Router) {

	r.HandleFunc("/api/dns", securityCheck(true, http.HandlerFunc(getAllDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/nodes", securityCheck(false, http.HandlerFunc(getNodeDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/custom", securityCheck(false, http.HandlerFunc(getCustomDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}", securityCheck(false, http.HandlerFunc(getDNS))).Methods("GET")
	r.HandleFunc("/api/dns/{network}", securityCheck(false, http.HandlerFunc(createDNS))).Methods("POST")
	r.HandleFunc("/api/dns/adm/pushdns", securityCheck(false, http.HandlerFunc(pushDNS))).Methods("POST")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheck(false, http.HandlerFunc(deleteDNS))).Methods("DELETE")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheck(false, http.HandlerFunc(updateDNS))).Methods("PUT")
}

//Gets all nodes associated with network, including pending nodes
func getNodeDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)

	dns, err := GetNodeDNS(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

//Gets all nodes associated with network, including pending nodes
func getAllDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dns, err := GetAllDNS()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	//Returns all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

func GetAllDNS() ([]models.DNSEntry, error) {
	var dns []models.DNSEntry
	networks, err := models.GetNetworks()
	if err != nil {
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

//Gets all nodes associated with network, including pending nodes
func getCustomDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)

	dns, err := GetCustomDNS(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
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

func SetDNS() error {
	hostfile := txeh.Hosts{}
	var corefilestring string
	networks, err := models.GetNetworks()
	if err != nil {
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
	err = functions.SetCorefile(corefilestring)

	return err
}

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

//Gets all nodes associated with network, including pending nodes
func getDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)

	dns, err := GetDNS(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
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

func createDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var entry models.DNSEntry
	var params = mux.Vars(r)

	//get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&entry)
	entry.Network = params["network"]

	err := ValidateDNSCreate(entry)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	entry, err = CreateDNS(entry)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = SetDNS()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

func updateDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var entry models.DNSEntry

	//start here
	entry, err := GetDNSEntry(params["domain"], params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	var dnschange models.DNSEntry

	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&dnschange)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	//fill in any missing fields
	if dnschange.Name == "" {
		dnschange.Name = entry.Name
	}
	if dnschange.Network == "" {
		dnschange.Network = entry.Network
	}
	if dnschange.Address == "" {
		dnschange.Address = entry.Address
	}

	err = ValidateDNSUpdate(dnschange, entry)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	entry, err = UpdateDNS(dnschange, entry)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	err = SetDNS()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	json.NewEncoder(w).Encode(entry)
}

func deleteDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	err := DeleteDNS(params["domain"], params["network"])

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	entrytext := params["domain"] + "." + params["network"]
	functions.PrintUserLog("netmaker", "deleted dns entry: "+entrytext, 1)
	err = SetDNS()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	json.NewEncoder(w).Encode(entrytext + " deleted.")
}

func CreateDNS(entry models.DNSEntry) (models.DNSEntry, error) {

	data, err := json.Marshal(&entry)
	if err != nil {
		return models.DNSEntry{}, err
	}
	key, err := functions.GetRecordKey(entry.Name, entry.Network)
	if err != nil {
		return models.DNSEntry{}, err
	}
	err = database.Insert(key, string(data), database.DNS_TABLE_NAME)

	return entry, err
}

func GetDNSEntry(domain string, network string) (models.DNSEntry, error) {
	var entry models.DNSEntry
	key, err := functions.GetRecordKey(domain, network)
	if err != nil {
		return entry, err
	}
	record, err := database.FetchRecord(database.DNS_TABLE_NAME, key)
	if err != nil {
		return entry, err
	}
	err = json.Unmarshal([]byte(record), &entry)
	return entry, err
}

func UpdateDNS(dnschange models.DNSEntry, entry models.DNSEntry) (models.DNSEntry, error) {

	key, err := functions.GetRecordKey(entry.Name, entry.Network)
	if err != nil {
		return entry, err
	}
	if dnschange.Name != "" {
		entry.Name = dnschange.Name
	}
	if dnschange.Address != "" {
		entry.Address = dnschange.Address
	}
	newkey, err := functions.GetRecordKey(entry.Name, entry.Network)

	err = database.DeleteRecord(database.DNS_TABLE_NAME, key)
	if err != nil {
		return entry, err
	}

	data, err := json.Marshal(&entry)
	err = database.Insert(newkey, string(data), database.DNS_TABLE_NAME)
	return entry, err

}

func DeleteDNS(domain string, network string) error {
	key, err := functions.GetRecordKey(domain, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.DNS_TABLE_NAME, key)
	return err
}

func pushDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	err := SetDNS()

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	log.Println("pushed DNS updates to nameserver")
	json.NewEncoder(w).Encode("DNS Pushed to CoreDNS")
}

func ValidateDNSCreate(entry models.DNSEntry) error {

	v := validator.New()

	_ = v.RegisterValidation("name_unique", func(fl validator.FieldLevel) bool {
		num, err := GetDNSEntryNum(entry.Name, entry.Network)
		return err == nil && num == 0
	})

	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		_, err := functions.GetParentNetwork(entry.Network)
		return err == nil
	})

	err := v.Struct(entry)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}

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
		_, err := functions.GetParentNetwork(change.Network)
		fmt.Println(err, entry.Network)
		return err == nil
	})

	//	_ = v.RegisterValidation("name_valid", func(fl validator.FieldLevel) bool {
	//		isvalid := functions.NameInDNSCharSet(entry.Name)
	//		notEmptyCheck := entry.Name != ""
	//		return isvalid && notEmptyCheck
	//	})
	//
	//	_ = v.RegisterValidation("address_valid", func(fl validator.FieldLevel) bool {
	//		isValid := true
	//		if entry.Address != "" {
	//			isValid = functions.IsIpNet(entry.Address)
	//		}
	//		return isValid
	//	})

	err := v.Struct(change)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}
