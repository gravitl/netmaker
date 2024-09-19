package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(deleteTag))).
		Methods(http.MethodDelete)

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
	var req models.CreateTagReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	user, err := logic.GetUser(r.Header.Get("user"))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	go func() {
		for _, hostID := range req.TaggedHosts {
			h, err := logic.GetHost(hostID)
			if err != nil {
				continue
			}
			if h.Tags == nil {
				h.Tags = make(map[models.TagID]struct{})
			}
			h.Tags[req.ID] = struct{}{}
			logic.UpsertHost(h)
		}
	}()

	tag := models.Tag{
		ID:        req.ID,
		CreatedBy: user.UserName,
		CreatedAt: time.Now(),
	}
	err = logic.InsertTag(tag)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, req, "created tag successfully")
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
	tag, err := logic.GetTag(updateTag.ID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	updateTag.NewID = models.TagID(strings.TrimSpace(updateTag.NewID.String()))
	if updateTag.NewID.String() != "" {
		tag.ID = updateTag.NewID
		err = logic.InsertTag(tag)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	go logic.UpdateTag(updateTag)
	logic.ReturnSuccessResponse(w, r, "updating tags")
}

// @Summary     Delete Tag
// @Router      /api/v1/tags [delete]
// @Tags        TAG
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deleteTag(w http.ResponseWriter, r *http.Request) {
	tagID, _ := url.QueryUnescape(r.URL.Query().Get("tag_id"))
	if tagID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err := logic.DeleteTag(models.TagID(tagID))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "deleted tag "+tagID)
}
