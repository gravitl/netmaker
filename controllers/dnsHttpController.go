package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func dnsHandlers(r *mux.Router) {

	r.HandleFunc("/api/dns", securityCheckDNS(true, true, http.HandlerFunc(getAllDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/nodes", securityCheckDNS(false, true, http.HandlerFunc(getNodeDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/custom", securityCheckDNS(false, true, http.HandlerFunc(getCustomDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}", securityCheckDNS(false, true, http.HandlerFunc(getDNS))).Methods("GET")
	r.HandleFunc("/api/dns/{network}", securityCheckDNS(false, false, http.HandlerFunc(createDNS))).Methods("POST")
	r.HandleFunc("/api/dns/adm/pushdns", securityCheckDNS(false, false, http.HandlerFunc(pushDNS))).Methods("POST")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheckDNS(false, false, http.HandlerFunc(deleteDNS))).Methods("DELETE")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheckDNS(false, false, http.HandlerFunc(updateDNS))).Methods("PUT")
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

// GetAllDNS - gets all dns entries
func GetAllDNS() ([]models.DNSEntry, error) {
	var dns []models.DNSEntry
	networks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.DNSEntry{}, err
	}
	for _, net := range networks {
		netdns, err := logic.GetDNS(net.NetID)
		if err != nil {
			return []models.DNSEntry{}, nil
		}
		dns = append(dns, netdns...)
	}
	return dns, nil
}

// GetNodeDNS - gets node dns
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

	dns, err := logic.GetCustomDNS(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// GetDNSEntryNum - gets which entry the dns was
func GetDNSEntryNum(domain string, network string) (int, error) {

	num := 0

	entries, err := logic.GetDNS(network)
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

// Gets all nodes associated with network, including pending nodes
func getDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)

	dns, err := logic.GetDNS(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
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
	err = logic.SetDNS()
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
	// fill in any missing fields
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
	err = logic.SetDNS()
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
	logger.Log(1, "deleted dns entry: ", entrytext)
	err = logic.SetDNS()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	json.NewEncoder(w).Encode(entrytext + " deleted.")
}

// CreateDNS - creates a DNS entry
func CreateDNS(entry models.DNSEntry) (models.DNSEntry, error) {

	data, err := json.Marshal(&entry)
	if err != nil {
		return models.DNSEntry{}, err
	}
	key, err := logic.GetRecordKey(entry.Name, entry.Network)
	if err != nil {
		return models.DNSEntry{}, err
	}
	err = database.Insert(key, string(data), database.DNS_TABLE_NAME)

	return entry, err
}

// GetDNSEntry - gets a DNS entry
func GetDNSEntry(domain string, network string) (models.DNSEntry, error) {
	var entry models.DNSEntry
	key, err := logic.GetRecordKey(domain, network)
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

// UpdateDNS - updates DNS entry
func UpdateDNS(dnschange models.DNSEntry, entry models.DNSEntry) (models.DNSEntry, error) {

	key, err := logic.GetRecordKey(entry.Name, entry.Network)
	if err != nil {
		return entry, err
	}
	if dnschange.Name != "" {
		entry.Name = dnschange.Name
	}
	if dnschange.Address != "" {
		entry.Address = dnschange.Address
	}
	newkey, err := logic.GetRecordKey(entry.Name, entry.Network)

	err = database.DeleteRecord(database.DNS_TABLE_NAME, key)
	if err != nil {
		return entry, err
	}

	data, err := json.Marshal(&entry)
	err = database.Insert(newkey, string(data), database.DNS_TABLE_NAME)
	return entry, err
}

// DeleteDNS - deletes a DNS entry
func DeleteDNS(domain string, network string) error {
	key, err := logic.GetRecordKey(domain, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.DNS_TABLE_NAME, key)
	return err
}

func pushDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	err := logic.SetDNS()

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "pushed DNS updates to nameserver")
	json.NewEncoder(w).Encode("DNS Pushed to CoreDNS")
}

// ValidateDNSCreate - checks if an entry is valid
func ValidateDNSCreate(entry models.DNSEntry) error {

	v := validator.New()

	_ = v.RegisterValidation("name_unique", func(fl validator.FieldLevel) bool {
		num, err := GetDNSEntryNum(entry.Name, entry.Network)
		return err == nil && num == 0
	})

	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		_, err := logic.GetParentNetwork(entry.Network)
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
		_, err := logic.GetParentNetwork(change.Network)
		if err != nil {
			logger.Log(0, err.Error())
		}
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
			logger.Log(1, e.Error())
		}
	}
	return err
}

//Security check DNS is middleware for every DNS function and just checks to make sure that its the master or dns token calling
//Only admin should have access to all these network-level actions
//DNS token should have access to only read functions
func securityCheckDNS(reqAdmin bool, allowDNSToken bool, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "W1R3: It's not you it's me.",
		}

		var params = mux.Vars(r)
		bearerToken := r.Header.Get("Authorization")
		if allowDNSToken && authenticateDNSToken(bearerToken) {
			r.Header.Set("user", "nameserver")
			networks, _ := json.Marshal([]string{ALL_NETWORK_ACCESS})
			r.Header.Set("networks", string(networks))
			next.ServeHTTP(w, r)
		} else {
			err, networks, username := SecurityCheck(reqAdmin, params["networkname"], bearerToken)
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					errorResponse.Code = http.StatusNotFound
				}
				errorResponse.Message = err.Error()
				returnErrorResponse(w, r, errorResponse)
				return
			}
			networksJson, err := json.Marshal(&networks)
			if err != nil {
				errorResponse.Message = err.Error()
				returnErrorResponse(w, r, errorResponse)
				return
			}
			r.Header.Set("user", username)
			r.Header.Set("networks", string(networksJson))
			next.ServeHTTP(w, r)
		}
	}
}
