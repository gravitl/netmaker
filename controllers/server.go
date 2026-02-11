package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"
	ch "github.com/gravitl/netmaker/clickhouse"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

var cpuProfileLog *os.File

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
		"/api/server/shutdown", logic.SecurityCheck(true,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				msg := "received api call to shutdown server, sending interruption..."
				slog.Warn(msg)
				_, _ = w.Write([]byte(msg))
				w.WriteHeader(http.StatusOK)
				_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
			})),
	).Methods(http.MethodPost)
	r.HandleFunc("/api/server/getconfig", allowUsers(http.HandlerFunc(getConfig))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/server/settings", allowUsers(http.HandlerFunc(getSettings))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/server/settings", logic.SecurityCheck(true, http.HandlerFunc(updateSettings))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/server/getserverinfo", logic.SecurityCheck(true, http.HandlerFunc(getServerInfo))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/server/status", getStatus).Methods(http.MethodGet)
	r.HandleFunc("/api/server/usage", logic.SecurityCheck(false, http.HandlerFunc(getUsage))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/server/cpu_profile", logic.SecurityCheck(false, http.HandlerFunc(cpuProfile))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/server/mem_profile", logic.SecurityCheck(false, http.HandlerFunc(memProfile))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/server/feature_flags", getFeatureFlags).Methods(http.MethodGet)
}

func cpuProfile(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("action") == "start"
	if start {
		os.Remove("/root/data/cpu.prof")
		cpuProfileLog = logic.StartCPUProfiling()
	} else {
		if cpuProfileLog != nil {
			logic.StopCPUProfiling(cpuProfileLog)
			cpuProfileLog = nil
		}
	}
}
func memProfile(w http.ResponseWriter, r *http.Request) {
	os.Remove("/root/data/mem.prof")
	logic.StartMemProfiling()
}

func getUsage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.SuccessResponse{
		Code:     http.StatusOK,
		Response: logic.GetCurrentServerUsage(),
	})
}

// @Summary     Get the server status
// @Router      /api/server/status [get]
// @Tags        Server
// @Produce     json
// @Success     200 {object} object "Server status"
func getStatus(w http.ResponseWriter, r *http.Request) {
	type status struct {
		DB               bool      `json:"db_connected"`
		Broker           bool      `json:"broker_connected"`
		IsBrokerConnOpen bool      `json:"is_broker_conn_open"`
		LicenseError     string    `json:"license_error"`
		IsPro            bool      `json:"is_pro"`
		DeploymentMode   string    `json:"deployment_mode"`
		TrialEndDate     time.Time `json:"trial_end_date"`
		IsOnTrialLicense bool      `json:"is_on_trial_license"`
		Version          string    `json:"version"`
	}

	licenseErr := ""
	if servercfg.ErrLicenseValidation != nil {
		licenseErr = servercfg.ErrLicenseValidation.Error()
	}
	//var trialEndDate time.Time
	//var err error
	// isOnTrial := false
	// if servercfg.IsPro &&
	// 	(servercfg.GetLicenseKey() == "" || servercfg.GetNetmakerTenantID() == "") {
	// 	trialEndDate, err = logic.GetTrialEndDate()
	// 	if err != nil {
	// 		slog.Error("failed to get trial end date", "error", err)
	// 	} else {
	// 		isOnTrial = true
	// 	}
	// }
	currentServerStatus := status{
		DB:               database.IsConnected(),
		Broker:           mq.IsConnected(),
		IsBrokerConnOpen: mq.IsConnectionOpen(),
		LicenseError:     licenseErr,
		IsPro:            servercfg.IsPro,
		DeploymentMode:   logic.GetDeploymentMode(),
		Version:          servercfg.Version,
		//TrialEndDate:     trialEndDate,
		//IsOnTrialLicense: isOnTrial,
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
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.ServerConfig
func getServerInfo(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	json.NewEncoder(w).Encode(logic.GetServerInfo())
	// w.WriteHeader(http.StatusOK)
}

// @Summary     Get the server configuration
// @Router      /api/server/getconfig [get]
// @Tags        Server
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.ServerConfig
func getConfig(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	scfg := logic.GetServerConfig()
	scfg.IsPro = "no"
	if servercfg.IsPro {
		scfg.IsPro = "yes"
	}

	scfg.ClientID = logic.Mask()
	scfg.ClientSecret = logic.Mask()
	json.NewEncoder(w).Encode(scfg)
	// w.WriteHeader(http.StatusOK)
}

// @Summary     Get the server settings
// @Router      /api/server/settings [get]
// @Tags        Server
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.ServerSettings
func getSettings(w http.ResponseWriter, r *http.Request) {
	scfg := logic.GetServerSettings()
	if scfg.ClientSecret != "" {
		scfg.ClientSecret = logic.Mask()
	}

	logic.ReturnSuccessResponseWithJson(w, r, scfg, "fetched server settings successfully")
}

// @Summary     Update the server settings
// @Router      /api/server/settings [put]
// @Tags        Server
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.ServerSettings true "Server settings"
// @Success     200 {object} models.ServerSettings
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateSettings(w http.ResponseWriter, r *http.Request) {
	var req models.ServerSettings
	force := r.URL.Query().Get("force")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err := logic.ValidateNewSettings(req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid settings: %w", err), "badrequest"))
		return
	}
	currSettings := logic.GetServerSettings()

	if req.AuthProvider != currSettings.AuthProvider && req.AuthProvider == "" {
		superAdmin, err := logic.GetSuperAdmin()
		if err != nil {
			err = fmt.Errorf("failed to get super admin: %v", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}

		if superAdmin.AuthType == models.OAuth {
			err := fmt.Errorf(
				"cannot remove IdP integration because an OAuth user has the super-admin role; transfer the super-admin role to another user first",
			)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	if currSettings.EnableFlowLogs != req.EnableFlowLogs {
		if req.EnableFlowLogs {
			err := ch.Initialize()
			if err != nil {
				err = fmt.Errorf("failed to enable flow logs: %v", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
				return
			}
			logic.StartFlowCleanupLoop()
		} else {
			logic.StopFlowCleanupLoop()
			ch.Close()
		}
	}

	err := logic.UpsertServerSettings(req)
	if err != nil {
		if req.EnableFlowLogs {
			logic.StopFlowCleanupLoop()
			ch.Close()
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to update server settings "+err.Error()), "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: identifySettingsUpdateAction(currSettings, req),
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   models.SettingSub.String(),
			Name: models.SettingSub.String(),
			Type: models.SettingSub,
		},
		Diff: models.Diff{
			Old: currSettings,
			New: req,
		},
		Origin: models.Dashboard,
	})
	go reInit(currSettings, req, force == "true")
	logic.ReturnSuccessResponseWithJson(w, r, req, "updated server settings successfully")
}

func reInit(curr, new models.ServerSettings, force bool) {
	logic.SettingsMutex.Lock()
	defer logic.SettingsMutex.Unlock()
	logic.ResetAuthProvider()
	logic.EmailInit()
	logic.SetVerbosity(int(logic.GetServerSettings().Verbosity))
	logic.ResetIDPSyncHook()
	if curr.MetricInterval != new.MetricInterval {
		logic.GetMetricsMonitor().Stop()
		logic.GetMetricsMonitor().Start()
	}

	if curr.EnableFlowLogs != new.EnableFlowLogs {
		go mq.PublishExporterFeatureFlags()
	}

	// On force AutoUpdate change, change AutoUpdate for all hosts.
	// On force FlowLogs enable, enable FlowLogs for all hosts.
	// On FlowLogs disable, forced or not, disable FlowLogs for all hosts.
	if force || !new.EnableFlowLogs {
		if curr.NetclientAutoUpdate != new.NetclientAutoUpdate ||
			curr.EnableFlowLogs != new.EnableFlowLogs {
			hosts, _ := logic.GetAllHosts()
			for _, host := range hosts {
				if curr.NetclientAutoUpdate != new.NetclientAutoUpdate {
					host.AutoUpdate = new.NetclientAutoUpdate
				}
				if curr.EnableFlowLogs != new.EnableFlowLogs {
					host.EnableFlowLogs = new.EnableFlowLogs
				}
				logic.UpsertHost(&host)
				mq.HostUpdate(&models.HostUpdate{
					Action: models.UpdateHost,
					Host:   host,
				})
			}
		}
	}
	if new.CleanUpInterval != curr.CleanUpInterval {
		logic.RestartHook("network-hook", time.Duration(new.CleanUpInterval)*time.Minute)
	}
	go mq.PublishPeerUpdate(false)
}

func identifySettingsUpdateAction(old, new models.ServerSettings) models.Action {
	// TODO: here we are relying on the dashboard to only
	// make singular updates, but it's possible that the
	// API can be called to make multiple changes to the
	// server settings. We should update it to log multiple
	// events or create singular update APIs.
	if old.MFAEnforced != new.MFAEnforced {
		if new.MFAEnforced {
			return models.EnforceMFA
		} else {
			return models.UnenforceMFA
		}
	}

	if old.BasicAuth != new.BasicAuth {
		if new.BasicAuth {
			return models.EnableBasicAuth
		} else {
			return models.DisableBasicAuth
		}
	}

	if old.Telemetry != new.Telemetry {
		if new.Telemetry == "off" {
			return models.DisableTelemetry
		} else {
			return models.EnableTelemetry
		}
	}

	if old.EnableFlowLogs != new.EnableFlowLogs {
		if new.EnableFlowLogs {
			return models.EnableFlowLogs
		} else {
			return models.DisableFlowLogs
		}
	}

	if old.NetclientAutoUpdate != new.NetclientAutoUpdate ||
		old.RacRestrictToSingleNetwork != new.RacRestrictToSingleNetwork ||
		old.ManageDNS != new.ManageDNS ||
		old.DefaultDomain != new.DefaultDomain ||
		old.EndpointDetection != new.EndpointDetection {
		return models.UpdateClientSettings
	}

	if old.AllowedEmailDomains != new.AllowedEmailDomains ||
		old.JwtValidityDuration != new.JwtValidityDuration {
		return models.UpdateAuthenticationSecuritySettings
	}

	if old.Verbosity != new.Verbosity ||
		old.MetricsPort != new.MetricsPort ||
		old.MetricInterval != new.MetricInterval ||
		old.AuditLogsRetentionPeriodInDays != new.AuditLogsRetentionPeriodInDays {
		return models.UpdateMonitoringAndDebuggingSettings
	}

	if old.EmailSenderAddr != new.EmailSenderAddr ||
		old.EmailSenderUser != new.EmailSenderUser ||
		old.EmailSenderPassword != new.EmailSenderPassword ||
		old.SmtpHost != new.SmtpHost ||
		old.SmtpPort != new.SmtpPort {
		return models.UpdateSMTPSettings
	}

	if old.AuthProvider != new.AuthProvider ||
		old.OIDCIssuer != new.OIDCIssuer ||
		old.ClientID != new.ClientID ||
		old.ClientSecret != new.ClientSecret ||
		old.SyncEnabled != new.SyncEnabled ||
		old.IDPSyncInterval != new.IDPSyncInterval ||
		old.GoogleAdminEmail != new.GoogleAdminEmail ||
		old.GoogleSACredsJson != new.GoogleSACredsJson ||
		old.AzureTenant != new.AzureTenant ||
		!cmp.Equal(old.GroupFilters, new.GroupFilters) ||
		cmp.Equal(old.UserFilters, new.UserFilters) {
		return models.UpdateIDPSettings
	}

	return models.Update
}

// @Summary     Get feature flags for this server
// @Router      /api/server/feature_flags [get]
// @Tags        Server
// @Produce     json
// @Success     200 {object} models.FeatureFlags
func getFeatureFlags(w http.ResponseWriter, r *http.Request) {
	logic.ReturnSuccessResponseWithJson(w, r, logic.GetFeatureFlags(), "")
}
