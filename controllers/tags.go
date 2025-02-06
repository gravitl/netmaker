package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
)

func tagHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(getTags))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(createTag))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(updateTag))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/v1/tags", logic.SecurityCheck(true, http.HandlerFunc(deleteTag))).
		Methods(http.MethodDelete)

}

// @Summary     List Tags in a network
// @Router      /api/v1/tags [get]
// @Tags        TAG
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getTags(w http.ResponseWriter, r *http.Request) {
	netID, _ := url.QueryUnescape(r.URL.Query().Get("network"))
	if netID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network id param is missing"), "badrequest"))
		return
	}
	// check if network exists
	_, err := logic.GetNetwork(netID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	tags, err := logic.ListTagsWithNodes(models.NetworkID(netID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get all network tag entries: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.SortTagEntrys(tags[:])
	logic.ReturnSuccessResponseWithJson(w, r, tags, "fetched all tags in the network "+netID)
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
	// check if tag network exists
	_, err = logic.GetNetwork(req.Network.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to get network details for "+req.Network.String()), "badrequest"))
		return
	}
	// check if tag exists
	tag := models.Tag{
		ID:        models.TagID(fmt.Sprintf("%s.%s", req.Network, req.TagName)),
		TagName:   req.TagName,
		Network:   req.Network,
		CreatedBy: user.UserName,
		ColorCode: req.ColorCode,
		CreatedAt: time.Now(),
	}
	_, err = logic.GetTag(tag.ID)
	if err == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("tag with id %s exists already", tag.TagName), "badrequest"))
		return
	}
	// validate name
	err = logic.CheckIDSyntax(tag.TagName)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.InsertTag(tag)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	go func() {
		for _, node := range req.TaggedNodes {
			if node.IsStatic {
				extclient, err := logic.GetExtClient(node.StaticNode.ClientID, node.StaticNode.Network)
				if err == nil && extclient.RemoteAccessClientID == "" {
					if extclient.Tags == nil {
						extclient.Tags = make(map[models.TagID]struct{})
					}
					extclient.Tags[tag.ID] = struct{}{}
					logic.SaveExtClient(&extclient)
				}
				continue
			}
			node, err := logic.GetNodeByID(node.ID)
			if err != nil {
				continue
			}
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			node.Tags[tag.ID] = struct{}{}
			logic.UpsertNode(&node)
		}
	}()
	go mq.PublishPeerUpdate(false)

	var res models.TagListRespNodes = models.TagListRespNodes{
		Tag:         tag,
		UsedByCnt:   len(req.TaggedNodes),
		TaggedNodes: req.TaggedNodes,
	}

	logic.ReturnSuccessResponseWithJson(w, r, res, "created tag successfully")
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
	updateTag.NewName = strings.TrimSpace(updateTag.NewName)
	var newID models.TagID
	if updateTag.NewName != "" {
		// validate name
		err = logic.CheckIDSyntax(updateTag.NewName)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		newID = models.TagID(fmt.Sprintf("%s.%s", tag.Network, updateTag.NewName))
		tag.ID = newID
		tag.TagName = updateTag.NewName
		err = logic.InsertTag(tag)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		// delete old Tag entry
		logic.DeleteTag(updateTag.ID, false)
	}
	if updateTag.ColorCode != "" && updateTag.ColorCode != tag.ColorCode {
		tag.ColorCode = updateTag.ColorCode
		err = logic.InsertTag(tag)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	go func() {
		logic.UpdateTag(updateTag, newID)
		if updateTag.NewName != "" {
			logic.UpdateDeviceTag(updateTag.ID, newID, tag.Network)
		}
		mq.PublishPeerUpdate(false)
	}()

	var res models.TagListRespNodes = models.TagListRespNodes{
		Tag:         tag,
		UsedByCnt:   len(updateTag.TaggedNodes),
		TaggedNodes: updateTag.TaggedNodes,
	}

	logic.ReturnSuccessResponseWithJson(w, r, res, "updated tags")
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
	tag, err := logic.GetTag(models.TagID(tagID))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// check if active policy is using the tag
	if logic.CheckIfTagAsActivePolicy(tag.ID, tag.Network) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tag is currently in use by an active policy"), "badrequest"))
		return
	}
	err = logic.DeleteTag(models.TagID(tagID), true)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	go func() {
		logic.RemoveDeviceTagFromAclPolicies(tag.ID, tag.Network)
		logic.RemoveTagFromEnrollmentKeys(tag.ID)
		mq.PublishPeerUpdate(false)
	}()
	logic.ReturnSuccessResponse(w, r, "deleted tag "+tagID)
}
