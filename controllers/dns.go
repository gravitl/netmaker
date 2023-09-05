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

	r.HandleFunc("/api/dns", logic.SecurityCheck(true, http.HandlerFunc(getAllDNS))).Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}/nodes", logic.SecurityCheck(true, http.HandlerFunc(getNodeDNS))).Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}/custom", logic.SecurityCheck(true, http.HandlerFunc(getCustomDNS))).Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}", logic.SecurityCheck(true, http.HandlerFunc(getDNS))).Methods(http.MethodGet)
	r.HandleFunc("/api/dns/{network}", logic.SecurityCheck(true, http.HandlerFunc(createDNS))).Methods(http.MethodPost)
	r.HandleFunc("/api/dns/adm/pushdns", logic.SecurityCheck(true, http.HandlerFunc(pushDNS))).Methods(http.MethodPost)
	r.HandleFunc("/api/dns/{network}/{domain}", logic.SecurityCheck(true, http.HandlerFunc(deleteDNS))).Methods(http.MethodDelete)
}

// swagger:route GET /api/dns/adm/{network}/nodes dns getNodeDNS
//
// Gets node DNS entries associated with a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
func getNodeDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetNodeDNS(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get node DNS entries for network [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route GET /api/dns dns getAllDNS
//
// Gets all DNS entries.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//	  		200: dnsResponse
func getAllDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dns, err := logic.GetAllDNS()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get all DNS entries: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.SortDNSEntrys(dns[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route GET /api/dns/adm/{network}/custom dns getCustomDNS
//
// Gets custom DNS entries associated with a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//	  		200: dnsResponse
func getCustomDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetCustomDNS(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get custom DNS entries for network [%s]: %v", network, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route GET /api/dns/adm/{network} dns getDNS
//
// Gets all DNS entries associated with the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//	  		200: dnsResponse
func getDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetDNS(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get all DNS entries for network [%s]: %v", network, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// swagger:route POST /api/dns/{network} dns createDNS
//
// Create a DNS entry.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//	  		200: dnsResponse
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
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	entry, err = logic.CreateDNS(entry)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to create DNS entry %+v: %v", entry, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	err = logic.SetDNS()
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to set DNS entries on file: %v", err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "new DNS record added:", entry.Name)
	if servercfg.IsMessageQueueBackend() {
		go func() {
			if err = mq.PublishPeerUpdate(); err != nil {
				logger.Log(0, "failed to publish peer update after ACL update on", entry.Network)
			}
			if err := mq.PublishCustomDNS(&entry); err != nil {
				logger.Log(0, "error publishing custom dns", err.Error())
			}
		}()
	}
	logger.Log(2, r.Header.Get("user"),
		fmt.Sprintf("DNS entry is set: %+v", entry))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

// swagger:route DELETE /api/dns/{network}/{domain} dns deleteDNS
//
// Delete a DNS entry.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: stringJSONResponse
//				*: stringJSONResponse
func deleteDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	entrytext := params["domain"] + "." + params["network"]
	err := logic.DeleteDNS(params["domain"], params["network"])

	if err != nil {
		logger.Log(0, "failed to delete dns entry: ", entrytext)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "deleted dns entry: ", entrytext)
	err = logic.SetDNS()
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to set DNS entries on file: %v", err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	json.NewEncoder(w).Encode(entrytext + " deleted.")
	go func() {
		dns := models.DNSUpdate{
			Action: models.DNSDeleteByName,
			Name:   entrytext,
		}
		if err := mq.PublishDNSUpdate(params["network"], dns); err != nil {
			logger.Log(0, "failed to publish dns update", err.Error())
		}
	}()

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
// Push DNS entries to nameserver.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: dnsStringJSONResponse
//				*: dnsStringJSONResponse
func pushDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	err := logic.SetDNS()

	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to set DNS entries on file: %v", err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "pushed DNS updates to nameserver")
	json.NewEncoder(w).Encode("DNS Pushed to CoreDNS")
}
