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
	t.Run("GetGroupValidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)

		decoder := json.NewDecoder(response.Body)
		for decoder.More() {
			var group models.Group
			err := decoder.Decode(&group)
			assert.Nil(t, err, err)
			Groups = append(Groups, group)
		}
		t.Log(Groups)
	})
	t.Run("GetGroupInvalidToken", func(t *testing.T) {
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
