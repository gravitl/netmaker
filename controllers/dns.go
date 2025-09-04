package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
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
	r.HandleFunc("/api/v1/nameserver", logic.SecurityCheck(true, http.HandlerFunc(createNs))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/nameserver", logic.SecurityCheck(true, http.HandlerFunc(listNs))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/nameserver", logic.SecurityCheck(true, http.HandlerFunc(updateNs))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/nameserver", logic.SecurityCheck(true, http.HandlerFunc(deleteNs))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/nameserver/global", logic.SecurityCheck(true, http.HandlerFunc(getGlobalNs))).Methods(http.MethodGet)
}

// @Summary     List Global Nameservers
// @Router      /api/v1/nameserver/global [get]
// @Tags        Auth
// @Accept      json
// @Param       query network string
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getGlobalNs(w http.ResponseWriter, r *http.Request) {

	logic.ReturnSuccessResponseWithJson(w, r, logic.GlobalNsList, "fetched nameservers")
}

// @Summary     Create Nameserver
// @Router      /api/v1/nameserver [post]
// @Tags        DNS
// @Accept      json
// @Param       body body models.NameserverReq
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createNs(w http.ResponseWriter, r *http.Request) {

	var req schema.Nameserver
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err := logic.ValidateNameserverReq(req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.Tags == nil {
		req.Tags = make(datatypes.JSONMap)
	}
	if req.Nodes == nil {
		req.Nodes = make(datatypes.JSONMap)
	}
	if gNs, ok := logic.GlobalNsList[req.Name]; ok {
		req.Servers = gNs.IPs
	}
	if !servercfg.IsPro {
		req.Tags = datatypes.JSONMap{
			"*": struct{}{},
		}
	}
	if req.MatchAll {
		req.MatchDomains = []string{"."}
	}
	ns := schema.Nameserver{
		ID:           uuid.New().String(),
		Name:         req.Name,
		NetworkID:    req.NetworkID,
		Description:  req.Description,
		MatchAll:     req.MatchAll,
		MatchDomains: req.MatchDomains,
		Servers:      req.Servers,
		Tags:         req.Tags,
		Nodes:        req.Nodes,
		Status:       true,
		CreatedBy:    r.Header.Get("user"),
		CreatedAt:    time.Now().UTC(),
	}

	err = ns.Create(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating nameserver "+err.Error()), logic.Internal),
		)
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   ns.ID,
			Name: ns.Name,
			Type: models.NameserverSub,
		},
		NetworkID: models.NetworkID(ns.NetworkID),
		Origin:    models.Dashboard,
	})

	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, ns, "created nameserver")
}

// @Summary     List Nameservers
// @Router      /api/v1/nameserver [get]
// @Tags        Auth
// @Accept      json
// @Param       query network string
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func listNs(w http.ResponseWriter, r *http.Request) {

	network := r.URL.Query().Get("network")
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	ns := schema.Nameserver{NetworkID: network}
	list, err := ns.ListByNetwork(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error listing nameservers "+err.Error()), "internal"),
		)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, list, "fetched nameservers")
}

// @Summary     Update Nameserver
// @Router      /api/v1/nameserver [put]
// @Tags        Auth
// @Accept      json
// @Param       body body models.NameserverReq
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateNs(w http.ResponseWriter, r *http.Request) {

	var updateNs schema.Nameserver
	err := json.NewDecoder(r.Body).Decode(&updateNs)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if err := logic.ValidateNameserverReq(updateNs); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if updateNs.Tags == nil {
		updateNs.Tags = make(datatypes.JSONMap)
	}
	if updateNs.Nodes == nil {
		updateNs.Nodes = make(datatypes.JSONMap)
	}

	ns := schema.Nameserver{ID: updateNs.ID}
	err = ns.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var updateStatus bool
	var updateMatchAll bool
	if updateNs.Status != ns.Status {
		updateStatus = true
	}
	if updateNs.MatchAll != ns.MatchAll {
		updateMatchAll = true
	}
	event := &models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   ns.ID,
			Name: updateNs.Name,
			Type: models.NameserverSub,
		},
		Diff: models.Diff{
			Old: ns,
			New: updateNs,
		},
		NetworkID: models.NetworkID(ns.NetworkID),
		Origin:    models.Dashboard,
	}
	ns.Servers = updateNs.Servers
	ns.Tags = updateNs.Tags
	ns.MatchDomains = updateNs.MatchDomains
	ns.MatchAll = updateNs.MatchAll
	ns.Description = updateNs.Description
	ns.Name = updateNs.Name
	ns.Nodes = updateNs.Nodes
	ns.Status = updateNs.Status
	ns.UpdatedAt = time.Now().UTC()

	err = ns.Update(db.WithContext(context.TODO()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating egress resource"+err.Error()), "internal"),
		)
		return
	}
	if updateStatus {
		ns.UpdateStatus(db.WithContext(context.TODO()))
	}
	if updateMatchAll {
		ns.UpdateMatchAll(db.WithContext(context.TODO()))
	}
	logic.LogEvent(event)
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, ns, "updated nameserver")
}

// @Summary     Delete Nameserver Resource
// @Router      /api/v1/nameserver [delete]
// @Tags        Auth
// @Accept      json
// @Param       body body models.Egress
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteNs(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")
	if id == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("id is required"), "badrequest"))
		return
	}
	ns := schema.Nameserver{ID: id}
	err := ns.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	err = ns.Delete(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   ns.ID,
			Name: ns.Name,
			Type: models.NameserverSub,
		},
		NetworkID: models.NetworkID(ns.NetworkID),
		Origin:    models.Dashboard,
	})

	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted nameserver resource")
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
	// check if default domain is appended if not append
	if logic.GetDefaultDomain() != "" &&
		!strings.HasSuffix(entry.Name, logic.GetDefaultDomain()) {
		entry.Name += "." + logic.GetDefaultDomain()
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

	if logic.GetManageDNS() {
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

	if logic.GetManageDNS() {
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
	if !logic.GetManageDNS() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("manage DNS is set to false"), "badrequest"),
		)
		return
	}
	var params = mux.Vars(r)
	netID := params["network"]
	k, err := logic.GetDNS(netID)
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
