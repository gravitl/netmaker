package errors

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

type ApiRespErr string

const (
	Internal     ApiRespErr = "internal"
	BadRequest   ApiRespErr = "badrequest"
	NotFound     ApiRespErr = "notfound"
	UnAuthorized ApiRespErr = "unauthorized"
	Forbidden    ApiRespErr = "forbidden"
	Unavailable  ApiRespErr = "unavailable"
)

// FormatError - formats into api error resp
func FormatError(err error, errType ApiRespErr) models.ErrorResponse {

	var status = http.StatusInternalServerError
	switch errType {
	case Internal:
		status = http.StatusInternalServerError
	case BadRequest:
		status = http.StatusBadRequest
	case NotFound:
		status = http.StatusNotFound
	case UnAuthorized:
		status = http.StatusUnauthorized
	case Forbidden:
		status = http.StatusForbidden
	case Unavailable:
		status = http.StatusServiceUnavailable
	default:
		status = http.StatusInternalServerError
	}

	var response = models.ErrorResponse{
		Code: status,
	}
	if err != nil {
		response.Message = err.Error()
	}
	return response
}
