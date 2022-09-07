package controller

import (
	"encoding/json"
	"fmt"
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

// swagger:route GET /api/dns/adm/{network}/nodes dns getNodeDNS
//
// Gets node DNS entries associated with a network
//
//		Schemes: https
//
// 		Security:
//   		oauth
func getNodeDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetNodeDNS(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get node DNS entries for network [%s]: %v", network, err))
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route GET /api/dns dns getAllDNS
//
// Gets all DNS entries
//
//		Schemes: https
//
// 		Security:
//   		oauth
func getAllDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dns, err := logic.GetAllDNS()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get all DNS entries: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route GET /api/dns/adm/{network}/custom dns getCustomDNS
//
// Gets custom DNS entries associated with a network
//
//		Schemes: https
//
// 		Security:
//   		oauth
func getCustomDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetCustomDNS(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get custom DNS entries for network [%s]: %v", network, err.Error()))
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route GET /api/dns/adm/{network} dns getDNS
//
// Gets all DNS entries associated with the network
//
//		Schemes: https
//
// 		Security:
//   		oauth
func getDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetDNS(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get all DNS entries for network [%s]: %v", network, err.Error()))
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route POST /api/dns/{network} dns createDNS
//
// Create a DNS entry
//
//		Schemes: https
//
// 		Security:
//   		oauth
func createDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var entry models.DNSEntry
	var params = mux.Vars(r)

	_ = json.NewDecoder(r.Body).Decode(&entry)
	entry.Network = params["network"]

	err := logic.ValidateDNSCreate(entry)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("invalid DNS entry %+v: %v", entry, err))
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	entry, err = CreateDNS(entry)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to create DNS entry %+v: %v", entry, err))
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = logic.SetDNS()
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to set DNS entries on file: %v", err))
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
			if err = mq.PublishPeerUpdate(&serverNode, false); err != nil {
				logger.Log(0, "failed to publish peer update after ACL update on", entry.Network)
			}
		}
	}
	logger.Log(2, r.Header.Get("user"),
		fmt.Sprintf("DNS entry is set: %+v", entry))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

// swagger:route DELETE /api/dns/{network}/{domain} dns deleteDNS
//
// Delete a DNS entry
//
//		Schemes: https
//
// 		Security:
//   		oauth
func deleteDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	entrytext := params["domain"] + "." + params["network"]
	err := logic.DeleteDNS(params["domain"], params["network"])

	if err != nil {
		logger.Log(0, "failed to delete dns entry: ", entrytext)
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, "deleted dns entry: ", entrytext)
	err = logic.SetDNS()
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to set DNS entries on file: %v", err))
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

// swagger:route POST /api/dns/adm/pushdns dns pushDNS
//
// Push DNS entries to nameserver
//
//		Schemes: https
//
// 		Security:
//   		oauth
func pushDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	err := logic.SetDNS()

	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to set DNS entries on file: %v", err))
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "pushed DNS updates to nameserver")
	json.NewEncoder(w).Encode("DNS Pushed to CoreDNS")
}
