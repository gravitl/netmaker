package controller

import (
    "github.com/gravitl/netmaker/models"
    "encoding/json"
    "net/http"
    "fmt"
)

func formatError(err error, errType string) models.ErrorResponse {

	var status = http.StatusInternalServerError
	switch errType {
	case "internal":
		status = http.StatusInternalServerError
	case "badrequest":
		status  = http.StatusBadRequest
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
                Code: status,
        }
	return response
}

func returnSuccessResponse(response http.ResponseWriter, request *http.Request, errorMesage models.ErrorResponse) {

}

func returnErrorResponse(response http.ResponseWriter, request *http.Request, errorMessage models.ErrorResponse) {
        httpResponse := &models.ErrorResponse{Code: errorMessage.Code, Message: errorMessage.Message}
        jsonResponse, err := json.Marshal(httpResponse)
        if err != nil {
                panic(err)
        }
	fmt.Println(errorMessage)
        response.Header().Set("Content-Type", "application/json")
        response.WriteHeader(errorMessage.Code)
        response.Write(jsonResponse)
}
