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

var networks []models.network

func TestCreatenetwork(t *testing.T) {
	network := models.network{}
	network.NetID = "skynet"
	network.AddressRange = "10.71.0.0/16"
	deletenetworks(t)
	t.Run("Createnetwork", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, "http://localhost:8081/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, "http://localhost:8081/api/networks", "badkey")
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
	t.Run("Duplicatenetwork", func(t *testing.T) {
		//issue #42
		t.Skip()
	})
}

func TestGetnetworks(t *testing.T) {
	t.Run("ValidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&networks)
		assert.Nil(t, err, err)
	})
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks", "badkey")
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

func TestGetnetwork(t *testing.T) {
	t.Run("ValidToken", func(t *testing.T) {
		var network models.network
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&network)
		assert.Nil(t, err, err)
		assert.Equal(t, "skynet", network.DisplayName)
	})
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/skynet", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("Invalidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/badnetwork", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestGetnetworkNodeNumber(t *testing.T) {
	t.Run("ValidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/skynet/numnodes", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message int
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		//assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/skynet/numnodes", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("Badnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/badnetwork/numnodes", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestDeletenetwork(t *testing.T) {
	t.Run("InvalidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/skynet", "badkey")
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
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message mongo.DeleteResult
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, int64(1), message.DeletedCount)

	})
	t.Run("Badnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/badnetwork", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("NodesExist", func(t *testing.T) {
		t.Skip()
	})
	//Create network for follow-on tests
	createnetwork(t)
}

func TestCreateAccessKey(t *testing.T) {
	key := models.AccessKey{}
	key.Name = "skynet"
	key.Uses = 10
	t.Run("MultiUse", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/networks/skynet/keys", "secretkey")
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
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/networks/skynet/keys", "secretkey")
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
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/networks/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		deleteKey(t, key.Name, "skynet")
	})

	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/networks/skynet/keys", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("Badnetwork", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/networks/badnetwork/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestDeleteKey(t *testing.T) {
	t.Run("KeyValid", func(t *testing.T) {
		//fails -- deletecount not returned
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/skynet/keys/skynet", "secretkey")
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
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/skynet/keys/badkey", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This key does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("KeyInValidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/badnetwork/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/networks/skynet/keys/skynet", "badkey")
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
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var keys []models.AccessKey
		err = json.NewDecoder(response.Body).Decode(&keys)
		assert.Nil(t, err, err)
	})
	//deletekeys
	t.Run("Invalidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/badnetwork/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/networks/skynet/keys", "badkey")
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

func TestUpdatenetwork(t *testing.T) {
	var returnednetwork models.network
	t.Run("UpdateNetID", func(t *testing.T) {
		type network struct {
			NetID string
		}
		var network network
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.NetID, returnednetwork.NetID)
	})
	t.Run("NetIDInvalidCredentials", func(t *testing.T) {
		type network struct {
			NetID string
		}
		var network network
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "badkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("Invalidnetwork", func(t *testing.T) {
		type network struct {
			NetID string
		}
		var network network
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/badnetwork", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, message.Code)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("UpdateNetIDTooLong", func(t *testing.T) {
		type network struct {
			NetID string
		}
		var network network
		network.NetID = "wirecat-skynet"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateAddress", func(t *testing.T) {
		type network struct {
			AddressRange string
		}
		var network network
		network.AddressRange = "10.0.0.1/24"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.AddressRange, returnednetwork.AddressRange)
	})
	t.Run("UpdateAddressInvalid", func(t *testing.T) {
		type network struct {
			AddressRange string
		}
		var network network
		network.AddressRange = "10.0.0.1/36"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateDisplayName", func(t *testing.T) {
		type network struct {
			DisplayName string
		}
		var network network
		network.DisplayName = "wirecat"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DisplayName, returnednetwork.DisplayName)

	})
	t.Run("UpdateDisplayNameInvalidName", func(t *testing.T) {
		type network struct {
			DisplayName string
		}
		var network network
		//create name that is longer than 100 chars
		name := ""
		for i := 0; i < 101; i++ {
			name = name + "a"
		}
		network.DisplayName = name
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DisplayName' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateInterface", func(t *testing.T) {
		type network struct {
			DefaultInterface string
		}
		var network network
		network.DefaultInterface = "netmaker"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultInterface, returnednetwork.DefaultInterface)

	})
	t.Run("UpdateListenPort", func(t *testing.T) {
		type network struct {
			DefaultListenPort int32
		}
		var network network
		network.DefaultListenPort = 6000
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultListenPort, returnednetwork.DefaultListenPort)
	})
	t.Run("UpdateListenPortInvalidPort", func(t *testing.T) {
		type network struct {
			DefaultListenPort int32
		}
		var network network
		network.DefaultListenPort = 1023
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultListenPort' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdatePostUP", func(t *testing.T) {
		type network struct {
			DefaultPostUp string
		}
		var network network
		network.DefaultPostUp = "sudo wg add-conf wc-netmaker /etc/wireguard/peers/conf"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultPostUp, returnednetwork.DefaultPostUp)
	})
	t.Run("UpdatePreUP", func(t *testing.T) {
		type network struct {
			DefaultPreUp string
		}
		var network network
		network.DefaultPreUp = "test string"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultPreUp, returnednetwork.DefaultPreUp)
	})
	t.Run("UpdateKeepAlive", func(t *testing.T) {
		type network struct {
			DefaultKeepalive int32
		}
		var network network
		network.DefaultKeepalive = 60
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultKeepalive, returnednetwork.DefaultKeepalive)
	})
	t.Run("UpdateKeepAliveTooBig", func(t *testing.T) {
		type network struct {
			DefaultKeepAlive int32
		}
		var network network
		network.DefaultKeepAlive = 1001
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
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
		type network struct {
			DefaultSaveConfig *bool
		}
		var network network
		value := false
		network.DefaultSaveConfig = &value
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, *network.DefaultSaveConfig, *returnednetwork.DefaultSaveConfig)
	})
	t.Run("UpdateManualSignUP", func(t *testing.T) {
		t.Skip()
		type network struct {
			AllowManualSignUp *bool
		}
		var network network
		value := true
		network.AllowManualSignUp = &value
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, *network.AllowManualSignUp, *returnednetwork.AllowManualSignUp)
	})
	t.Run("DefaultCheckInterval", func(t *testing.T) {
		type network struct {
			DefaultCheckInInterval int32
		}
		var network network
		network.DefaultCheckInInterval = 6000
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultCheckInInterval, returnednetwork.DefaultCheckInInterval)
	})
	t.Run("DefaultCheckIntervalTooBig", func(t *testing.T) {
		type network struct {
			DefaultCheckInInterval int32
		}
		var network network
		network.DefaultCheckInInterval = 100001
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultCheckInInterval' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("MultipleFields", func(t *testing.T) {
		type network struct {
			DisplayName       string
			DefaultListenPort int32
		}
		var network network
		network.DefaultListenPort = 7777
		network.DisplayName = "multi"
		response, err := api(t, network, http.MethodPut, "http://localhost:8081/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnednetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DisplayName, returnednetwork.DisplayName)
		assert.Equal(t, network.DefaultListenPort, returnednetwork.DefaultListenPort)
	})
}
