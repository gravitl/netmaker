package logic

import (
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

type ApiErrorType string

const (
	Internal     ApiErrorType = "internal"
	BadReq       ApiErrorType = "badrequest"
	NotFound     ApiErrorType = "notfound"
	UnAuthorized ApiErrorType = "unauthorized"
	Forbidden    ApiErrorType = "forbidden"
)

// FormatError - takes ErrorResponse and uses correct code
func FormatError(err error, errType ApiErrorType) models.ErrorResponse {

	var status = http.StatusInternalServerError
	switch errType {
	case Internal:
		status = http.StatusInternalServerError
	case BadReq:
		status = http.StatusBadRequest
	case NotFound:
		status = http.StatusNotFound
	case UnAuthorized:
		status = http.StatusUnauthorized
	case Forbidden:
		status = http.StatusForbidden
	default:
		status = http.StatusInternalServerError
	}

	var response = models.ErrorResponse{
		Message: err.Error(),
		Code:    status,
	}
	return response
}

// ReturnSuccessResponse - processes message and adds header
func ReturnSuccessResponse(response http.ResponseWriter, request *http.Request, message string) {
	var httpResponse models.SuccessResponse
	httpResponse.Code = http.StatusOK
	httpResponse.Message = message
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	json.NewEncoder(response).Encode(httpResponse)
}

// ReturnSuccessResponseWithJson - processes message and adds header
func ReturnSuccessResponseWithJson(response http.ResponseWriter, request *http.Request, res interface{}, message string) {
	var httpResponse models.SuccessResponse
	httpResponse.Code = http.StatusOK
	httpResponse.Response = res
	httpResponse.Message = message
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	json.NewEncoder(response).Encode(httpResponse)
}

// ReturnErrorResponse - processes error and adds header
func ReturnErrorResponse(response http.ResponseWriter, request *http.Request, errorMessage models.ErrorResponse) {
	httpResponse := &models.ErrorResponse{Code: errorMessage.Code, Message: errorMessage.Message}
	jsonResponse, err := json.Marshal(httpResponse)
	if err != nil {
		panic(err)
	}
	slog.Debug("processed request error", "err", errorMessage.Message)
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(errorMessage.Code)
	response.Write(jsonResponse)
}
