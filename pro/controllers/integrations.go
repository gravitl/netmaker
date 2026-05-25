package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/grpc/siem"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/pro/integration"
	"github.com/gravitl/netmaker/schema"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func IntegrationHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/integrations/{type}", logic.SecurityCheck(true, http.HandlerFunc(getIntegration))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/integrations/{type}/{id}", logic.SecurityCheck(true, http.HandlerFunc(upsertIntegration))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/integrations/{type}/{id}", logic.SecurityCheck(true, http.HandlerFunc(deleteIntegration))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/integrations/{type}/{id}/test", logic.SecurityCheck(true, http.HandlerFunc(testIntegration))).Methods(http.MethodPost)
}

// extractAndValidateIntegration pulls {type} and {id} from the URL
// and validates both against the provider registry.
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
// @Router      /api/v1/integrations/{type}/{id} [get]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Param       type            path string true "Integration type (e.g. siem)"
// @Success     200 {object} schema.Integration
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getIntegration(w http.ResponseWriter, r *http.Request) {
	intType := integration.Type(mux.Vars(r)["type"])

	intg := &schema.Integration{
		Type: string(intType),
	}
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

	logic.ReturnSuccessResponseWithJson(w, r, integrations[0], "integration retrieved")
}

// @Summary     Upsert an integration
// @Router      /api/v1/integrations/{type}/{id} [put]
// @Tags        Integrations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       type            path  string             true "Integration type (e.g. siem)"
// @Param       id              path  string             true "Provider ID (e.g. splunk)"
// @Param       body            body  schema.Integration true "Integration config"
// @Success     200 {object} schema.Integration
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func upsertIntegration(w http.ResponseWriter, r *http.Request) {
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

	provider, _ := integration.Lookup(intType, id) // already validated above
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

	err = intg.Upsert(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	go func(configBytes json.RawMessage) {
		config := make(map[string]interface{})
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error unmarshaling config: %s", err.Error()))
			return
		}

		configStruct, err := structpb.NewStruct(config)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error constructing struct val: %s", err.Error()))
			return
		}

		err = siem.Client().Init(context.Background(), string(id), configStruct)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error upserting siem integration %s on exporter: %v", id, err))

			err = mq.PublishIntegrationUpsert(string(id))
			if err != nil {
				logger.Log(0, fmt.Sprintf("error publishing siem integration upsert event %s on exporter: %v", id, err))
			}
		}
	}(config)

	logic.ReturnSuccessResponseWithJson(w, r, intg, "integration saved")
}

// @Summary     Delete an integration
// @Router      /api/v1/integrations/{type}/{id} [delete]
// @Tags        Integrations
// @Security    oauth
// @Produce     json
// @Param       type            path string true "Integration type (e.g. siem)"
// @Param       id              path string true "Provider ID (e.g. splunk)"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteIntegration(w http.ResponseWriter, r *http.Request) {
	_, id, ok := extractAndValidateIntegration(w, r)
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

	go func() {
		err := siem.Client().Terminate(context.Background())
		if err != nil {
			logger.Log(0, fmt.Sprintf("error terminating siem integration %s on exporter: %v", id, err))

			err = mq.PublishIntegrationDelete(string(id))
			if err != nil {
				logger.Log(0, fmt.Sprintf("error publishing siem integration delete event %s on exporter: %v", id, err))
			}
		}
	}()

	logic.ReturnSuccessResponse(w, r, "integration deleted")
}

// @Summary     Test an integration config
// @Router      /api/v1/integrations/{type}/{id}/test [post]
// @Tags        Integrations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       type            path  string true "Integration type (e.g. siem)"
// @Param       id              path  string true "Provider ID (e.g. splunk)"
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

	provider, _ := integration.Lookup(intType, id) // already validated above
	err = provider.Validate(config)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	err = provider.Test(config)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("integration test failed: %w", err), logic.BadReq))
		return
	}

	logic.ReturnSuccessResponse(w, r, "integration test passed")
}
