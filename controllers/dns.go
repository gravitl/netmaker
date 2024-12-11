package controller

import (
	"encoding/json"
	"errors"
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

	r.HandleFunc("/api/dns", logic.SecurityCheck(true, http.HandlerFunc(getAllDNS))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}/nodes", logic.SecurityCheck(true, http.HandlerFunc(getNodeDNS))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}/custom", logic.SecurityCheck(true, http.HandlerFunc(getCustomDNS))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}", logic.SecurityCheck(true, http.HandlerFunc(getDNS))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/dns/adm/{network}/sync", logic.SecurityCheck(true, http.HandlerFunc(syncDNS))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/dns/{network}", logic.SecurityCheck(true, http.HandlerFunc(createDNS))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/dns/adm/pushdns", logic.SecurityCheck(true, http.HandlerFunc(pushDNS))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/dns/{network}/{domain}", logic.SecurityCheck(true, http.HandlerFunc(deleteDNS))).
		Methods(http.MethodDelete)
}

// @Summary     Gets node DNS entries associated with a network
// @Router      /api/dns/{network} [get]
// @Tags        DNS
// @Accept      json
// @Param       network path string true "Network identifier"
// @Success     200 {array} models.DNSEntry
// @Failure     500 {object} models.ErrorResponse
func getNodeDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetNodeDNS(models.NetworkID(network))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get node DNS entries for network [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// @Summary     Get all DNS entries
// @Router      /api/dns [get]
// @Tags        DNS
// @Accept      json
// @Success     200 {array} models.DNSEntry
// @Failure     500 {object} models.ErrorResponse
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

// @Summary     Gets custom DNS entries associated with a network
// @Router      /api/dns/adm/{network}/custom [get]
// @Tags        DNS
// @Accept      json
// @Param       network path string true "Network identifier"
// @Success     200 {array} models.DNSEntry
// @Failure     500 {object} models.ErrorResponse
func getCustomDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetCustomDNS(network)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"failed to get custom DNS entries for network [%s]: %v",
				network,
				err.Error(),
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// @Summary     Get all DNS entries associated with the network
// @Router      /api/dns/adm/{network} [get]
// @Tags        DNS
// @Accept      json
// @Param       network path string true "Network identifier"
// @Success     200 {array} models.DNSEntry
// @Failure     500 {object} models.ErrorResponse
func getDNS(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var dns []models.DNSEntry
	var params = mux.Vars(r)
	network := params["network"]
	dns, err := logic.GetDNS(models.NetworkID(network))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get all DNS entries for network [%s]: %v", network, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dns)
}

// @Summary     Create a new DNS entry
// @Router      /api/dns/adm/{network} [post]
// @Tags        DNS
// @Accept      json
// @Param       network path string true "Network identifier"
// @Param       body body models.DNSEntry true "DNS entry details"
// @Success     200 {object} models.DNSEntry
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var entry models.DNSEntry
	var params = mux.Vars(r)
	netID := params["network"]

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
	if servercfg.IsDNSMode() {
		err = logic.SetDNS()
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("Failed to set DNS entries on file: %v", err))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}

	if servercfg.GetManageDNS() {
		mq.SendDNSSyncByNetwork(netID)
	}

	logger.Log(1, "new DNS record added:", entry.Name)
	logger.Log(2, r.Header.Get("user"),
		fmt.Sprintf("DNS entry is set: %+v", entry))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

// @Summary     Delete a DNS entry
// @Router      /api/dns/{network}/{domain} [delete]
// @Tags        DNS
// @Accept      json
// @Param       network path string true "Network identifier"
// @Param       domain path string true "Domain Name"
// @Success     200 {array} models.DNSEntry
// @Failure     500 {object} models.ErrorResponse
func deleteDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	netID := params["network"]
	entrytext := params["domain"] + "." + params["network"]
	err := logic.DeleteDNS(params["domain"], params["network"])

	if err != nil {
		logger.Log(0, "failed to delete dns entry: ", entrytext)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "deleted dns entry: ", entrytext)
	if servercfg.IsDNSMode() {
		err = logic.SetDNS()
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("Failed to set DNS entries on file: %v", err))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}

	if servercfg.GetManageDNS() {
		mq.SendDNSSyncByNetwork(netID)
	}

	json.NewEncoder(w).Encode(entrytext + " deleted.")

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

// @Summary     Push DNS entries to nameserver
// @Router      /api/dns/adm/pushdns [post]
// @Tags        DNS
// @Accept      json
// @Success     200 {string} string "DNS Pushed to CoreDNS"
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func pushDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")
	if !servercfg.IsDNSMode() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("DNS Mode is set to off"), "badrequest"),
		)
		return
	}
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

// @Summary     Sync DNS entries for a given network
// @Router      /api/dns/adm/{network}/sync [post]
// @Tags        DNS
// @Accept      json
// @Success     200 {string} string "DNS Sync completed successfully"
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func syncDNS(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")
	if !servercfg.GetManageDNS() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("manage DNS is set to false"), "badrequest"),
		)
		return
	}
	var params = mux.Vars(r)
	netID := params["network"]
	k, err := logic.GetDNS(models.NetworkID(netID))
	if err == nil && len(k) > 0 {
		err = mq.PushSyncDNS(k)
	}

	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("Failed to Sync DNS entries to network %s: %v", netID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "DNS Sync complelted successfully")
	json.NewEncoder(w).Encode("DNS Sync completed successfully")
}
