package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

func dnsHandlers(r *mux.Router) {

	r.HandleFunc("/api/dns", securityCheck(true, http.HandlerFunc(getAllDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/nodes", securityCheck(false, http.HandlerFunc(getNodeDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/custom", securityCheck(false, http.HandlerFunc(getCustomDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}", securityCheck(false, http.HandlerFunc(getDNS))).Methods("GET")
	r.HandleFunc("/api/dns/{network}", securityCheck(false, http.HandlerFunc(createDNS))).Methods("POST")
	r.HandleFunc("/api/dns/adm/pushdns", securityCheck(false, http.HandlerFunc(pushDNS))).Methods("POST")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheck(false, http.HandlerFunc(deleteDNS))).Methods("DELETE")
}

//Gets all nodes associated with network, including pending nodes
func getNodeDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)

	dns, err := logic.GetNodeDNS(params["network"])
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
	dns, err := logic.GetAllDNS()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	//Returns all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
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

	err := logic.ValidateDNSCreate(entry)
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
	logger.Log(1, "new DNS record added:", entry.Name)
	if servercfg.IsMessageQueueBackend() {
		serverNode, err := logic.GetNetworkServerLocal(entry.Network)
		if err != nil {
			logger.Log(1, "failed to find server node after DNS update on", entry.Network)
		} else {
			if err = logic.ServerUpdate(&serverNode, false); err != nil {
				logger.Log(1, "failed to update server node after DNS update on", entry.Network)
			}
			if err = mq.PublishPeerUpdate(&serverNode); err != nil {
				logger.Log(0, "failed to publish peer update after ACL update on", entry.Network)
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

func deleteDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	err := logic.DeleteDNS(params["domain"], params["network"])

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
