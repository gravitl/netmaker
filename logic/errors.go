package logic

import (
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// FormatError - takes ErrorResponse and uses correct code
func FormatError(err error, errType string) models.ErrorResponse {

	var status = http.StatusInternalServerError
	switch errType {
	case "internal":
		status = http.StatusInternalServerError
	case "badrequest":
		status = http.StatusBadRequest
	case "notfound":
		status = http.StatusNotFound
	case "unauthorized":
		status = http.StatusUnauthorized
	case "forbidden":
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

// ReturnErrorResponse - processes error and adds header
func ReturnErrorResponse(response http.ResponseWriter, request *http.Request, errorMessage models.ErrorResponse) {
	httpResponse := &models.ErrorResponse{Code: errorMessage.Code, Message: errorMessage.Message}
	jsonResponse, err := json.Marshal(httpResponse)
	if err != nil {
		panic(err)
	}
	logger.Log(1, "processed request error:", errorMessage.Message)
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(errorMessage.Code)
	response.Write(jsonResponse)
}
