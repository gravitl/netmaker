package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestFormatError(t *testing.T) {
	response := logic.FormatError(errors.New("this is a sample error"), "badrequest")
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "this is a sample error", response.Message)
}

func TestReturnSuccessResponse(t *testing.T) {
	var response models.SuccessResponse
	handler := func(rw http.ResponseWriter, r *http.Request) {
		logic.ReturnSuccessResponse(rw, r, "This is a test message")
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	//body, err := ioutil.ReadAll(resp.Body)
	//assert.Nil(t, err)
	//t.Log(body, string(body))
	err := json.NewDecoder(resp.Body).Decode(&response)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "This is a test message", response.Message)
}

func TestReturnErrorResponse(t *testing.T) {
	var response, errMessage models.ErrorResponse
	errMessage.Code = http.StatusUnauthorized
	errMessage.Message = "You are not authorized to access this endpoint"
	handler := func(rw http.ResponseWriter, r *http.Request) {
		logic.ReturnErrorResponse(rw, r, errMessage)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	err := json.NewDecoder(resp.Body).Decode(&response)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Equal(t, "You are not authorized to access this endpoint", response.Message)
}
