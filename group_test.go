package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

var Groups []models.Group

func TestGetGroups(t *testing.T) {
	t.Run("GetGroupsValidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&Groups)
		assert.Nil(t, err, err)
	})
	t.Run("GetGroupsInvalidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
}

func TestCreateGroup(t *testing.T) {
	group := models.Group{}
	group.NameID = "skynet"
	group.AddressRange = "10.71.0.0/16"
	t.Run("CreateGroup", func(t *testing.T) {
		response, err := api(t, group, http.MethodPost, "http://localhost:8081/api/groups", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("CreateGroupInvalidToken", func(t *testing.T) {
		response, err := api(t, group, http.MethodPost, "http://localhost:8081/api/groups", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("CreateGroupBadName", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
	t.Run("CreateGroupBadAddress", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
	t.Run("CreateGroupDuplicateGroup", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
}

func TestGetGroup(t *testing.T) {
	t.Run("GetGroupValidToken", func(t *testing.T) {
		var group models.Group
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&group)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DisplayName, "skynet")
	})
	t.Run("GetGroupInvalidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("GetGroupInvalidGroup", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/badgroup", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})

}
