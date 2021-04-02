package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

var Groups []models.Group

func TestCreateGroup(t *testing.T) {
	group := models.Group{}
	group.NameID = "skynet"
	group.AddressRange = "10.71.0.0/16"
	deleteGroups(t)
	t.Run("CreateGroup", func(t *testing.T) {
		response, err := api(t, group, http.MethodPost, "http://localhost:8081/api/groups", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("InvalidToken", func(t *testing.T) {
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
	t.Run("BadName", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
	t.Run("BadAddress", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
	t.Run("DuplicateGroup", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
}

func TestGetGroups(t *testing.T) {
	t.Run("ValidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&Groups)
		assert.Nil(t, err, err)
	})
	t.Run("InvalidToken", func(t *testing.T) {
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

func TestGetGroup(t *testing.T) {
	t.Run("ValidToken", func(t *testing.T) {
		var group models.Group
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&group)
		assert.Nil(t, err, err)
		assert.Equal(t, "skynet", group.DisplayName)
	})
	t.Run("InvalidToken", func(t *testing.T) {
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
	t.Run("InvalidGroup", func(t *testing.T) {
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

func TestGetGroupNodeNumber(t *testing.T) {
	t.Run("ValidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet/numnodes", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message int
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		//assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet/numnodes", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("BadGroup", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/badgroup/numnodes", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestDeleteGroup(t *testing.T) {
	t.Run("InvalidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/skynet", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("ValidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message mongo.DeleteResult
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, int64(1), message.DeletedCount)

	})
	t.Run("BadGroup", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/badgroup", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("NodesExist", func(t *testing.T) {
		t.Skip()
	})
	//Create Group for follow-on tests
	createGroup(t)
}

func TestCreateAccessKey(t *testing.T) {
	key := models.AccessKey{}
	key.Name = "skynet"
	key.Uses = 10
	t.Run("MultiUse", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/groups/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		message, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err, err)
		assert.NotNil(t, message, message)
		returnedkey := getKey(t, key.Name)
		assert.Equal(t, key.Name, returnedkey.Name)
		assert.Equal(t, key.Uses, returnedkey.Uses)
	})
	deleteKey(t, "skynet", "skynet")
	t.Run("ZeroUse", func(t *testing.T) {
		//t.Skip()
		key.Uses = 0
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/groups/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		message, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err, err)
		assert.NotNil(t, message, message)
		returnedkey := getKey(t, key.Name)
		assert.Equal(t, key.Name, returnedkey.Name)
		assert.Equal(t, 1, returnedkey.Uses)
	})
	t.Run("DuplicateAccessKey", func(t *testing.T) {
		//t.Skip()
		//this will fail
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/groups/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		deleteKey(t, key.Name, "skynet")
	})

	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/groups/skynet/keys", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("BadGroup", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/groups/badgroup/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestDeleteKey(t *testing.T) {
	t.Run("KeyValid", func(t *testing.T) {
		//fails -- deletecount not returned
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/skynet/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message mongo.DeleteResult
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, int64(1), message.DeletedCount)
	})
	t.Run("InValidKey", func(t *testing.T) {
		//fails -- status message  not returned
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/skynet/keys/badkey", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This key does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("KeyInValidGroup", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/badgroup/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/skynet/keys/skynet", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
}

func TestGetKeys(t *testing.T) {
	createKey(t)
	t.Run("Valid", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var keys []models.AccessKey
		err = json.NewDecoder(response.Body).Decode(&keys)
		assert.Nil(t, err, err)
	})
	//deletekeys
	t.Run("InvalidGroup", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/badgroup/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet/keys", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
}

func TestUpdateGroup(t *testing.T) {
	var returnedGroup models.Group
	t.Run("UpdateNameID", func(t *testing.T) {
		type Group struct {
			NameID string
		}
		var group Group
		group.NameID = "wirecat"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.NameID, returnedGroup.NameID)
	})
	t.Run("NameIDInvalidCredentials", func(t *testing.T) {
		type Group struct {
			NameID string
		}
		var group Group
		group.NameID = "wirecat"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "badkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("InvalidGroup", func(t *testing.T) {
		type Group struct {
			NameID string
		}
		var group Group
		group.NameID = "wirecat"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/badgroup", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, message.Code)
		assert.Equal(t, "W1R3: This group does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("UpdateNameIDTooLong", func(t *testing.T) {
		type Group struct {
			NameID string
		}
		var group Group
		group.NameID = "wirecat-skynet"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateAddress", func(t *testing.T) {
		type Group struct {
			AddressRange string
		}
		var group Group
		group.AddressRange = "10.0.0.1/24"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.AddressRange, returnedGroup.AddressRange)
	})
	t.Run("UpdateAddressInvalid", func(t *testing.T) {
		type Group struct {
			AddressRange string
		}
		var group Group
		group.AddressRange = "10.0.0.1/36"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateDisplayName", func(t *testing.T) {
		type Group struct {
			DisplayName string
		}
		var group Group
		group.DisplayName = "wirecat"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DisplayName, returnedGroup.DisplayName)

	})
	t.Run("UpdateDisplayNameInvalidName", func(t *testing.T) {
		type Group struct {
			DisplayName string
		}
		var group Group
		//create name that is longer than 100 chars
		name := ""
		for i := 0; i < 101; i++ {
			name = name + "a"
		}
		group.DisplayName = name
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DisplayName' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateInterface", func(t *testing.T) {
		type Group struct {
			DefaultInterface string
		}
		var group Group
		group.DefaultInterface = "netmaker"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DefaultInterface, returnedGroup.DefaultInterface)

	})
	t.Run("UpdateListenPort", func(t *testing.T) {
		type Group struct {
			DefaultListenPort int32
		}
		var group Group
		group.DefaultListenPort = 6000
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DefaultListenPort, returnedGroup.DefaultListenPort)
	})
	t.Run("UpdateListenPortInvalidPort", func(t *testing.T) {
		type Group struct {
			DefaultListenPort int32
		}
		var group Group
		group.DefaultListenPort = 1023
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultListenPort' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdatePostUP", func(t *testing.T) {
		type Group struct {
			DefaultPostUp string
		}
		var group Group
		group.DefaultPostUp = "sudo wg add-conf wc-netmaker /etc/wireguard/peers/conf"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DefaultPostUp, returnedGroup.DefaultPostUp)
	})
	t.Run("UpdatePreUP", func(t *testing.T) {
		type Group struct {
			DefaultPreUp string
		}
		var group Group
		group.DefaultPreUp = "test string"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DefaultPreUp, returnedGroup.DefaultPreUp)
	})
	t.Run("UpdateKeepAlive", func(t *testing.T) {
		type Group struct {
			DefaultKeepalive int32
		}
		var group Group
		group.DefaultKeepalive = 60
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DefaultKeepalive, returnedGroup.DefaultKeepalive)
	})
	t.Run("UpdateKeepAliveTooBig", func(t *testing.T) {
		type Group struct {
			DefaultKeepAlive int32
		}
		var group Group
		group.DefaultKeepAlive = 1001
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultKeepAlive' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateSaveConfig", func(t *testing.T) {
		//causes panic
		t.Skip()
		type Group struct {
			DefaultSaveConfig *bool
		}
		var group Group
		value := false
		group.DefaultSaveConfig = &value
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, *group.DefaultSaveConfig, *returnedGroup.DefaultSaveConfig)
	})
	t.Run("UpdateManualSignUP", func(t *testing.T) {
		t.Skip()
		type Group struct {
			AllowManualSignUp *bool
		}
		var group Group
		value := true
		group.AllowManualSignUp = &value
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, *group.AllowManualSignUp, *returnedGroup.AllowManualSignUp)
	})
	t.Run("DefaultCheckInterval", func(t *testing.T) {
		type Group struct {
			DefaultCheckInInterval int32
		}
		var group Group
		group.DefaultCheckInInterval = 6000
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DefaultCheckInInterval, returnedGroup.DefaultCheckInInterval)
	})
	t.Run("DefaultCheckIntervalTooBig", func(t *testing.T) {
		type Group struct {
			DefaultCheckInInterval int32
		}
		var group Group
		group.DefaultCheckInInterval = 100001
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultCheckInInterval' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("MultipleFields", func(t *testing.T) {
		type Group struct {
			DisplayName       string
			DefaultListenPort int32
		}
		var group Group
		group.DefaultListenPort = 7777
		group.DisplayName = "multi"
		response, err := api(t, group, http.MethodPut, "http://localhost:8081/api/groups/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedGroup)
		assert.Nil(t, err, err)
		assert.Equal(t, group.DisplayName, returnedGroup.DisplayName)
		assert.Equal(t, group.DefaultListenPort, returnedGroup.DefaultListenPort)
	})
}
