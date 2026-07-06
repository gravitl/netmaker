package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	grpcs "github.com/gravitl/netmaker/grpc/siem"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/pro/integration"
	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
	siempkg "github.com/gravitl/netmaker/pro/integration/siem"
	logic2 "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func IntegrationHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/integrations/mdm/providers", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listMDMProviders)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/integrations/mdm/sync", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(triggerMDMSync)))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/integrations/mdm/device_state", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listMDMDeviceState)))).Methods(http.MethodGet)

	r.HandleFunc("/api/v1/integrations/edr/providers", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listEDRProviders)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/integrations/edr/sync", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(triggerEDRSync)))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/integrations/edr/device_state", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listEDRDeviceState)))).Methods(http.MethodGet)

	r.HandleFunc("/api/v1/integrations/{type}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getIntegration)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/integrations/{type}/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(upsertIntegration)))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/integrations/{type}/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(deleteIntegration)))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/integrations/{type}/{id}/test", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(testIntegration)))).Methods(http.MethodPost)
}

func extractAndValidateIntegration(w http.ResponseWriter, r *http.Request) (integration.Type, integration.ProviderID, bool) {
	vars := mux.Vars(r)
	intType := integration.Type(vars["type"])
	id := integration.ProviderID(vars["id"])

	_, err := integration.Lookup(intType, id)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return "", "", false
	}
	return intType, id, true
}

// @Summary     Get an integration
// @Router      /api/v1/integrations/{type} [get]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Param       type            path string true "Integration type (e.g. siem, mdm)"
// @Success     200 {object} schema.Integration
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getIntegration(w http.ResponseWriter, r *http.Request) {
	intType := integration.Type(mux.Vars(r)["type"])
	if !integration.TypeExists(intType) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("unknown integration type '%s'", intType), logic.BadReq))
		return
	}

	intg := &schema.Integration{Type: string(intType)}
	integrations, err := intg.ListByType(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if len(integrations) == 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("integration not found"), logic.NotFound))
		return
	}

	if len(integrations) > 1 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("cannot have more than one integration of type %s", intType), logic.Internal))
		return
	}

	intg = &integrations[0]
	err = redactConfig(intg)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to redact integration config: %v", err), logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, intg, "integration retrieved")
}

// @Summary     Upsert an integration
// @Router      /api/v1/integrations/{type}/{id} [put]
// @Tags        Integrations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       type            path  string true "Integration type (e.g. siem, mdm)"
// @Param       id              path  string true "Provider ID (e.g. splunk, intune)"
// @Param       body            body  object true "Integration config"
// @Success     200 {object} schema.Integration
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func upsertIntegration(w http.ResponseWriter, r *http.Request) {
	intType, id, ok := extractAndValidateIntegration(w, r)
	if !ok {
		return
	}

	intg := &schema.Integration{Type: string(intType)}
	integrations, err := intg.ListByType(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if len(integrations) > 0 {
		var isUpsert bool
		if len(integrations) == 1 {
			existing := integrations[0]
			if existing.ID == string(id) && existing.Type == string(intType) {
				isUpsert = true
			}
		}

		if !isUpsert {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("cannot have more than one integration of type %s", intType), logic.BadReq))
			return
		}
	}

	var config json.RawMessage
	err = json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid request body: %w", err), logic.BadReq))
		return
	}

	if intType == integration.TypeMDM {
		config, err = mergeMDMConfig(r.Context(), string(id), config, len(integrations) == 1)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
			return
		}
	}
	if intType == integration.TypeEDR {
		config, err = mergeEDRConfig(r.Context(), string(id), config, len(integrations) == 1)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
			return
		}
	}

	provider, _ := integration.Lookup(intType, id)
	err = provider.Validate(config)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	intg = &schema.Integration{
		ID:     string(id),
		Type:   string(intType),
		Config: datatypes.JSON(config),
	}

	if intg.TenantID == "" {
		intg.TenantID = scope.ID(logic.DefaultScope(r.Context()))
	}
	err = intg.Upsert(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if intType == integration.TypeSIEM {
		go initSIEMExporter(string(id), config)
	}

	err = redactConfig(intg)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to redact integration config: %v", err), logic.Internal))
		return
	}

	logic2.PushToSIEM()
	logic.ReturnSuccessResponseWithJson(w, r, intg, "integration saved")
}

func initSIEMExporter(id string, configBytes json.RawMessage) {
	config := make(map[string]interface{})
	err := json.Unmarshal(configBytes, &config)
	if err != nil {
		logger.Log(0, fmt.Sprintf("error unmarshaling config: %s", err.Error()))
		return
	}

	configStruct, err := structpb.NewStruct(config)
	if err != nil {
		logger.Log(0, fmt.Sprintf("error constructing struct val: %s", err.Error()))
		return
	}

	err = grpcs.Client().Init(context.Background(), id, configStruct)
	if err != nil {
		logger.Log(0, fmt.Sprintf("error upserting siem integration %s on exporter: %v", id, err))

		err = mq.PublishIntegrationUpsert(id)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error publishing siem integration upsert event %s on exporter: %v", id, err))
		}
	}
}

// @Summary     Delete an integration
// @Router      /api/v1/integrations/{type}/{id} [delete]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Param       type            path string true "Integration type (e.g. siem, mdm)"
// @Param       id              path string true "Provider ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteIntegration(w http.ResponseWriter, r *http.Request) {
	intType, id, ok := extractAndValidateIntegration(w, r)
	if !ok {
		return
	}

	intg := &schema.Integration{ID: string(id)}
	err := intg.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("integration not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	err = intg.Delete(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if intType == integration.TypeSIEM {
		go func() {
			err := grpcs.Client().Terminate(context.Background())
			if err != nil {
				logger.Log(0, fmt.Sprintf("error terminating siem integration %s on exporter: %v", id, err))

				err = mq.PublishIntegrationDelete(string(id))
				if err != nil {
					logger.Log(0, fmt.Sprintf("error publishing siem integration delete event %s on exporter: %v", id, err))
				}
			}
		}()
	}

	logic2.SkipPushToSiem()
	logic.ReturnSuccessResponse(w, r, "integration deleted")
}

// @Summary     Test an integration config
// @Router      /api/v1/integrations/{type}/{id}/test [post]
// @Tags        Integrations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       type            path  string true "Integration type (e.g. siem, mdm)"
// @Param       id              path  string true "Provider ID"
// @Param       body            body  object true "Provider config to test (not saved)"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func testIntegration(w http.ResponseWriter, r *http.Request) {
	intType, id, ok := extractAndValidateIntegration(w, r)
	if !ok {
		return
	}

	var config json.RawMessage
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid request body: %w", err), logic.BadReq))
		return
	}

	if intType == integration.TypeMDM || intType == integration.TypeEDR {
		existing := &schema.Integration{ID: string(id)}
		hasExisting := existing.Get(r.Context()) == nil && existing.Type == string(intType)
		if intType == integration.TypeMDM {
			config, err = mergeMDMConfig(r.Context(), string(id), config, hasExisting)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
				return
			}
		}
		if intType == integration.TypeEDR {
			config, err = mergeEDRConfig(r.Context(), string(id), config, hasExisting)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
				return
			}
		}
	}

	provider, _ := integration.Lookup(intType, id)
	err = provider.Validate(config)
	if err != nil {
		if intType == integration.TypeMDM {
			logMDMVerifyEvent(r, string(id), false, err.Error())
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	err = provider.Test(config)
	if err != nil {
		if intType == integration.TypeMDM {
			logMDMVerifyEvent(r, string(id), false, err.Error())
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("integration test failed: %w", err), logic.BadReq))
		return
	}

	if intType == integration.TypeMDM {
		logMDMVerifyEvent(r, string(id), true, "")
	}

	logic.ReturnSuccessResponse(w, r, "integration test passed")
}

// @Summary     List built-in MDM provider types
// @Router      /api/v1/integrations/mdm/providers [get]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Success     200 {array} mdmpkg.ProviderType
func listMDMProviders(w http.ResponseWriter, r *http.Request) {
	logic.ReturnSuccessResponseWithJson(w, r, mdmpkg.ListProviderTypes(), "fetched mdm provider types")
}

// @Summary     Trigger an out-of-cycle MDM sync
// @Router      /api/v1/integrations/mdm/sync [post]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Success     202 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func triggerMDMSync(w http.ResponseWriter, r *http.Request) {
	active, err := mdmpkg.GetActive(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	if active == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no MDM integration configured"), logic.BadReq))
		return
	}

	syncCtx := db.WithContext(context.Background())
	go func() {
		if err := mdmpkg.RunMDMSyncForce(syncCtx); err != nil {
			logger.Log(0, "mdm: manual sync failed:", err.Error())
		}
	}()

	logic.LogEvent(&models.Event{
		Action:      schema.MDMSync,
		TriggeredBy: r.Header.Get("user"),
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		Target: models.Subject{
			ID:   active.ID,
			Name: active.ID,
			Type: schema.MDMSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			New: map[string]interface{}{"status": "queued", "provider": active.ID},
		},
	})
	logic.ReturnSuccessResponseWithJson(w, r, map[string]any{"queued": true}, "mdm sync queued")
}

// @Summary     List synced MDM device states
// @Router      /api/v1/integrations/mdm/device_state [get]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Param       host_id   query string false "Filter by host UUID"
// @Param       provider  query string false "Filter by provider name"
// @Success     200 {array} schema.DeviceMDMState
func listMDMDeviceState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hostID := r.URL.Query().Get("host_id")
	provider := r.URL.Query().Get("provider")
	state := &schema.DeviceMDMState{HostID: hostID, Provider: provider}
	var out []schema.DeviceMDMState
	var err error
	switch {
	case hostID != "" && provider != "":
		err = state.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("mdm device state not found"), logic.NotFound))
				return
			}
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
		out = []schema.DeviceMDMState{*state}
	case hostID != "":
		out, err = state.ListByHost(ctx)
	case provider != "":
		out, err = state.ListByProvider(ctx)
	default:
		out, err = state.ListAll(ctx)
	}
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, out, "fetched mdm device states")
}

func listEDRProviders(w http.ResponseWriter, r *http.Request) {
	logic.ReturnSuccessResponseWithJson(w, r, edrpkg.ListProviderTypes(), "fetched edr provider types")
}

func triggerEDRSync(w http.ResponseWriter, r *http.Request) {
	active, err := edrpkg.GetActive(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	if active == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no EDR integration configured"), logic.BadReq))
		return
	}
	syncCtx := db.WithContext(context.Background())
	go func() {
		if err := edrpkg.RunEDRSyncForce(syncCtx); err != nil {
			logger.Log(0, "edr: manual sync failed:", err.Error())
		}
	}()
	logic.ReturnSuccessResponseWithJson(w, r, map[string]any{"queued": true}, "edr sync queued")
}

func listEDRDeviceState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hostID := r.URL.Query().Get("host_id")
	provider := r.URL.Query().Get("provider")
	state := &schema.DeviceEDRState{HostID: hostID, Provider: provider}
	var out []schema.DeviceEDRState
	var err error
	switch {
	case hostID != "" && provider != "":
		err = state.Get(ctx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = logic.SyncHostEDRState(ctx, hostID)
			err = state.Get(ctx)
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("edr device state not found"), logic.NotFound))
				return
			}
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
		out = []schema.DeviceEDRState{*state}
	case hostID != "":
		out, err = state.ListByHost(ctx)
	case provider != "":
		out, err = state.ListByProvider(ctx)
	default:
		out, err = state.ListAll(ctx)
	}
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, out, "fetched edr device states")
}

func mergeEDRConfig(ctx context.Context, providerID string, incoming json.RawMessage, hasExisting bool) (json.RawMessage, error) {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(incoming, &patch); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	changed := false
	for _, field := range []string{"client_secret", "api_token", "password"} {
		merged, ok, err := mergeEDRSecretField(ctx, providerID, patch, field, hasExisting)
		if err != nil {
			return nil, err
		}
		if ok {
			patch = merged
			changed = true
		}
	}
	if !changed {
		return incoming, nil
	}
	return json.Marshal(patch)
}

func mergeEDRSecretField(
	ctx context.Context,
	providerID string,
	patch map[string]json.RawMessage,
	field string,
	hasExisting bool,
) (map[string]json.RawMessage, bool, error) {
	secret, ok := patch[field]
	if !ok {
		return patch, false, nil
	}
	var secretStr string
	if err := json.Unmarshal(secret, &secretStr); err != nil {
		return patch, false, nil
	}
	if !isMaskedSecret(secretStr) || !hasExisting {
		return patch, false, nil
	}
	existing := &schema.Integration{ID: providerID}
	if err := existing.Get(ctx); err != nil {
		return patch, false, nil
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(existing.Config, &stored); err != nil {
		return nil, false, err
	}
	prev, ok := stored[field]
	if !ok {
		return patch, false, nil
	}
	patch[field] = prev
	return patch, true, nil
}

func redactConfig(intg *schema.Integration) error {
	if intg.Type == string(integration.TypeMDM) {
		redacted, err := mdmpkg.RedactConfig(intg.ID, json.RawMessage(intg.Config))
		if err != nil {
			return err
		}
		intg.Config = datatypes.JSON(redacted)
		return nil
	}
	if intg.Type == string(integration.TypeEDR) {
		redacted, err := edrpkg.RedactConfig(intg.ID, json.RawMessage(intg.Config))
		if err != nil {
			return err
		}
		intg.Config = datatypes.JSON(redacted)
		return nil
	}

	switch integration.ProviderID(intg.ID) {
	case integration.ProviderDatadog:
		var config siempkg.DatadogConfig
		if err := json.Unmarshal(intg.Config, &config); err != nil {
			return err
		}
		config.APIKey = logic.Mask()
		configBytes, err := json.Marshal(config)
		if err != nil {
			return err
		}
		intg.Config = configBytes
	case integration.ProviderElastic:
		var config siempkg.ElasticConfig
		if err := json.Unmarshal(intg.Config, &config); err != nil {
			return err
		}
		if config.APIKey != "" {
			config.APIKey = logic.Mask()
		}
		if config.Password != "" {
			config.Password = logic.Mask()
		}
		configBytes, err := json.Marshal(config)
		if err != nil {
			return err
		}
		intg.Config = configBytes
	case integration.ProviderSentinel:
		var config siempkg.SentinelConfig
		if err := json.Unmarshal(intg.Config, &config); err != nil {
			return err
		}
		config.SharedKey = logic.Mask()
		configBytes, err := json.Marshal(config)
		if err != nil {
			return err
		}
		intg.Config = configBytes
	case integration.ProviderSplunk:
		var config siempkg.SplunkConfig
		if err := json.Unmarshal(intg.Config, &config); err != nil {
			return err
		}
		config.HECToken = logic.Mask()
		configBytes, err := json.Marshal(config)
		if err != nil {
			return err
		}
		intg.Config = configBytes
	}
	return nil
}

func mergeMDMConfig(ctx context.Context, providerID string, incoming json.RawMessage, hasExisting bool) (json.RawMessage, error) {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(incoming, &patch); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	changed := false
	for _, field := range []string{"client_secret", "api_token", "password"} {
		merged, ok, err := mergeMDMSecretField(ctx, providerID, patch, field, hasExisting)
		if err != nil {
			return nil, err
		}
		if ok {
			patch = merged
			changed = true
		}
	}
	if !changed {
		return incoming, nil
	}
	return json.Marshal(patch)
}

func mergeMDMSecretField(
	ctx context.Context,
	providerID string,
	patch map[string]json.RawMessage,
	field string,
	hasExisting bool,
) (map[string]json.RawMessage, bool, error) {
	secret, ok := patch[field]
	if !ok {
		return patch, false, nil
	}
	var secretStr string
	if err := json.Unmarshal(secret, &secretStr); err != nil {
		return patch, false, nil
	}
	if !isMaskedSecret(secretStr) || !hasExisting {
		return patch, false, nil
	}

	existing := &schema.Integration{ID: providerID}
	if err := existing.Get(ctx); err != nil {
		return patch, false, nil
	}

	var stored map[string]json.RawMessage
	if err := json.Unmarshal(existing.Config, &stored); err != nil {
		return nil, false, err
	}
	storedSecret, ok := stored[field]
	if !ok {
		return patch, false, nil
	}
	patch[field] = storedSecret
	return patch, true, nil
}

func isMaskedSecret(s string) bool {
	return s == logic.Mask() || s == "********"
}

func logMDMVerifyEvent(r *http.Request, providerID string, ok bool, errMsg string) {
	diff := map[string]interface{}{"status": "ok", "provider": providerID}
	if !ok {
		diff = map[string]interface{}{"status": "failed", "error": errMsg}
	}
	logic.LogEvent(&models.Event{
		Action:      schema.MDMVerify,
		TriggeredBy: r.Header.Get("user"),
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		Target: models.Subject{
			ID:   providerID,
			Name: providerID,
			Type: schema.MDMSub,
		},
		Origin: schema.Dashboard,
		Diff:   models.Diff{New: diff},
	})
}
