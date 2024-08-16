package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

func serverHandlers(r *mux.Router) {
	// r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(true, http.HandlerFunc(addNetwork))).Methods(http.MethodPost)
	r.HandleFunc(
		"/api/server/health",
		func(resp http.ResponseWriter, req *http.Request) {
			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("Server is up and running!!"))
		},
	).Methods(http.MethodGet)
	r.HandleFunc(
		"/api/server/shutdown",
		func(w http.ResponseWriter, _ *http.Request) {
			msg := "received api call to shutdown server, sending interruption..."
			slog.Warn(msg)
			_, _ = w.Write([]byte(msg))
			w.WriteHeader(http.StatusOK)
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		},
	).Methods(http.MethodPost)
	r.HandleFunc("/api/server/getconfig", allowUsers(http.HandlerFunc(getConfig))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/server/getserverinfo", logic.SecurityCheck(true, http.HandlerFunc(getServerInfo))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/server/status", getStatus).Methods(http.MethodGet)
	r.HandleFunc("/api/server/usage", logic.SecurityCheck(false, http.HandlerFunc(getUsage))).
		Methods(http.MethodGet)
}

func getUsage(w http.ResponseWriter, _ *http.Request) {
	type usage struct {
		Hosts            int `json:"hosts"`
		Clients          int `json:"clients"`
		Networks         int `json:"networks"`
		Users            int `json:"users"`
		Ingresses        int `json:"ingresses"`
		Egresses         int `json:"egresses"`
		Relays           int `json:"relays"`
		InternetGateways int `json:"internet_gateways"`
		FailOvers        int `json:"fail_overs"`
	}
	var serverUsage usage
	hosts, err := logic.GetAllHosts()
	if err == nil {
		serverUsage.Hosts = len(hosts)
	}
	clients, err := logic.GetAllExtClients()
	if err == nil {
		serverUsage.Clients = len(clients)
	}
	users, err := logic.GetUsers()
	if err == nil {
		serverUsage.Users = len(users)
	}
	networks, err := logic.GetNetworks()
	if err == nil {
		serverUsage.Networks = len(networks)
	}
	// TODO this part bellow can be optimized to get nodes just once
	ingresses, err := logic.GetAllIngresses()
	if err == nil {
		serverUsage.Ingresses = len(ingresses)
	}
	egresses, err := logic.GetAllEgresses()
	if err == nil {
		serverUsage.Egresses = len(egresses)
	}
	relays, err := logic.GetRelays()
	if err == nil {
		serverUsage.Relays = len(relays)
	}
	gateways, err := logic.GetInternetGateways()
	if err == nil {
		serverUsage.InternetGateways = len(gateways)
	}
	failOvers, err := logic.GetAllFailOvers()
	if err == nil {
		serverUsage.FailOvers = len(failOvers)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.SuccessResponse{
		Code:     http.StatusOK,
		Response: serverUsage,
	})
}

// @Summary     Get the server status
// @Router      /api/server/status [get]
// @Tags        Server
// @Security    oauth2
func getStatus(w http.ResponseWriter, r *http.Request) {
	// @Success     200 {object} status
	type status struct {
		DB               bool      `json:"db_connected"`
		Broker           bool      `json:"broker_connected"`
		IsBrokerConnOpen bool      `json:"is_broker_conn_open"`
		LicenseError     string    `json:"license_error"`
		IsPro            bool      `json:"is_pro"`
		TrialEndDate     time.Time `json:"trial_end_date"`
		IsOnTrialLicense bool      `json:"is_on_trial_license"`
	}

	licenseErr := ""
	if servercfg.ErrLicenseValidation != nil {
		licenseErr = servercfg.ErrLicenseValidation.Error()
	}
	var trialEndDate time.Time
	var err error
	isOnTrial := false
	if servercfg.IsPro &&
		(servercfg.GetLicenseKey() == "" || servercfg.GetNetmakerTenantID() == "") {
		trialEndDate, err = logic.GetTrialEndDate()
		if err != nil {
			slog.Error("failed to get trial end date", "error", err)
		} else {
			isOnTrial = true
		}
	}
	currentServerStatus := status{
		DB:               database.IsConnected(),
		Broker:           mq.IsConnected(),
		IsBrokerConnOpen: mq.IsConnectionOpen(),
		LicenseError:     licenseErr,
		IsPro:            servercfg.IsPro,
		TrialEndDate:     trialEndDate,
		IsOnTrialLicense: isOnTrial,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&currentServerStatus)
}

// allowUsers - allow all authenticated (valid) users - only used by getConfig, may be able to remove during refactor
func allowUsers(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errorResponse := models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: logic.Unauthorized_Msg,
		}
		bearerToken := r.Header.Get("Authorization")
		tokenSplit := strings.Split(bearerToken, " ")
		authToken := ""
		if len(tokenSplit) < 2 {
			logic.ReturnErrorResponse(w, r, errorResponse)
			return
		} else {
			authToken = tokenSplit[1]
		}
		user, _, _, err := logic.VerifyUserToken(authToken)
		if err != nil || user == "" {
			logic.ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// @Summary     Get the server information
// @Router      /api/server/getserverinfo [get]
// @Tags        Server
// @Security    oauth2
// @Success     200 {object} models.ServerConfig
func getServerInfo(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	json.NewEncoder(w).Encode(servercfg.GetServerInfo())
	// w.WriteHeader(http.StatusOK)
}

// @Summary     Get the server configuration
// @Router      /api/server/getconfig [get]
// @Tags        Server
// @Security    oauth2
// @Success     200 {object} config.ServerConfig
func getConfig(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	scfg := servercfg.GetServerConfig()
	scfg.IsPro = "no"
	if servercfg.IsPro {
		scfg.IsPro = "yes"
	}
	json.NewEncoder(w).Encode(scfg)
	// w.WriteHeader(http.StatusOK)
}
