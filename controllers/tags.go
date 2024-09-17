package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func tagHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(getAllTags))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(createTag))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(updateTag))).
		Methods(http.MethodPut)

}

// @Summary     Get all Tag entries
// @Router      /api/v1/tags [get]
// @Tags        TAG
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getAllTags(w http.ResponseWriter, r *http.Request) {
	tags, err := logic.ListTagsWithHosts()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get all DNS entries: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.SortTagEntrys(tags[:])
	logic.ReturnSuccessResponseWithJson(w, r, tags, "fetched all tags")
}

// @Summary     Create Tag
// @Router      /api/v1/tags [post]
// @Tags        TAG
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func createTag(w http.ResponseWriter, r *http.Request) {
	var tag models.Tag
	err := json.NewDecoder(r.Body).Decode(&tag)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.InsertTag(tag)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, tag, "created tag successfully")
}

// @Summary     Update Tag
// @Router      /api/v1/tags [put]
// @Tags        TAG
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func updateTag(w http.ResponseWriter, r *http.Request) {
	var updateTag models.UpdateTagReq
	err := json.NewDecoder(r.Body).Decode(&updateTag)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	go logic.UpdateTag(updateTag)
	logic.ReturnSuccessResponse(w, r, "updating tags")
}
