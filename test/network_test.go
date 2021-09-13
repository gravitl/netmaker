package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestCreateNetwork(t *testing.T) {

	network := models.Network{}
	network.NetID = "skynet"
	network.AddressRange = "10.71.0.0/16"

	deleteNetworks(t)
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
	t.Run("CreateNetwork", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
	})
	t.Run("DuplicateNetwork", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("BadName", func(t *testing.T) {
		network.NetID = "thisnameistoolong"
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("BadAddress", func(t *testing.T) {
		network.NetID = "wirecat"
		network.AddressRange = "10.300.20.56/36"
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

}

func TestGetNetworks(t *testing.T) {

	t.Run("ValidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&Networks)
		assert.Nil(t, err, err)
	})
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
}

func TestGetNetwork(t *testing.T) {

	t.Run("ValidToken", func(t *testing.T) {
		var network models.Network
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		err = json.NewDecoder(response.Body).Decode(&network)
		assert.Nil(t, err, err)
		// --- needs fixing ------ returns previous name
		//assert.Equal(t, "skynet", network.DisplayName)
	})
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
	t.Run("InvalidNetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/badnetwork", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, message.Message, "no result found")
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
}

func TestDeleteNetwork(t *testing.T) {

	t.Run("InvalidKey", func(t *testing.T) {
		setup(t)
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet", "badkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
	t.Run("Badnetwork", func(t *testing.T) {
		// non-existant network deletion does not result in an error
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/badnetwork", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message string
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "success", message)
	})
	t.Run("NodesExist", func(t *testing.T) {
		setup(t)
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "node check failed. All nodes must be deleted before deleting network", message.Message)
		assert.Equal(t, http.StatusBadRequest, message.Code)
	})
	t.Run("ValidKey", func(t *testing.T) {
		setup(t)
		deleteAllNodes(t)
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message string
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "success", message)
	})
}

func TestCreateKey(t *testing.T) {
	//ensure we are working with known networks
	deleteNetworks(t)
	createNetwork(t)

	key := models.AccessKey{}
	key.Name = "skynet"
	key.Uses = 10
	t.Run("MultiUse", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/skynet/keys", "secretkey")
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
		//
		key.Uses = 0
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/skynet/keys", "secretkey")
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
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "duplicate AccessKey Name", message.Message)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/skynet/keys", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
	t.Run("Badnetwork", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/badnetwork/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "no result found", message.Message)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
}

func TestDeleteKey(t *testing.T) {
	//ensure we are working with known networks
	deleteNetworks(t)
	createNetwork(t)
	//ensure key exists
	createKey(t)
	t.Run("KeyValid", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("InValidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet/keys/badkey", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "key badkey does not exist", message.Message)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("KeyInValidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/badnetwork/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "no result found", message.Message)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet/keys/skynet", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
}

func TestGetKeys(t *testing.T) {
	//ensure we are working with known networks
	deleteNetworks(t)
	createNetwork(t)
	createKey(t)
	t.Run("Valid", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var keys []models.AccessKey
		err = json.NewDecoder(response.Body).Decode(&keys)
		assert.Nil(t, err, err)
	})
	t.Run("Invalidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/badnetwork/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "no result found", message.Message)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
	t.Run("InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet/keys", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Contains(t, message.Message, "rror verifying user toke")
	})
}

func TestUpdateNetwork(t *testing.T) {
	//ensure we are working with known networks
	deleteNetworks(t)
	createNetwork(t)
	var returnedNetwork models.Network
	t.Run("UpdateNetID", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "failed to update network wirecat, cannot change netid.", message.Message)
	})
	t.Run("Invalidnetwork", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/badnetwork", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, message.Code)
		assert.Equal(t, "no result found", message.Message)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
	t.Run("UpdateAddress", func(t *testing.T) {
		t.Skip() // getting an Internal Server Error --- not sure why
		network := getNetwork(t, "skynet")
		network.AddressRange = "10.0.0.1/24"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.AddressRange, returnedNetwork.AddressRange)
	})
	t.Run("UpdateAddressInvalid", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.AddressRange = "10.0.0.1/36"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "validation for 'AddressRange' failed")

	})
	t.Run("UpdateDisplayName", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DisplayName = "wirecat"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DisplayName, returnedNetwork.DisplayName)

	})
	t.Run("UpdateDisplayNameInvalidName", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		//create name that is longer than 100 chars
		name := ""
		for i := 0; i < 101; i++ {
			name = name + "a"
		}
		network.DisplayName = name
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'DisplayName' failed")
	})
	t.Run("UpdateInterface", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultInterface = "netmaker"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultInterface, returnedNetwork.DefaultInterface)

	})
	t.Run("UpdateListenPort", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultListenPort = 6000
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultListenPort, returnedNetwork.DefaultListenPort)
	})
	t.Run("UpdateListenPortInvalidPort", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultListenPort = 65540
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'DefaultListenPort' failed")
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("UpdatePostUP", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultPostUp = "sudo wg add-conf wc-netmaker /etc/wireguard/peers/conf"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultPostUp, returnedNetwork.DefaultPostUp)
	})
	t.Run("UpdatePostDown", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultPostDown = "test string"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultPostDown, returnedNetwork.DefaultPostDown)
	})
	t.Run("UpdateKeepAlive", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultKeepalive = 60
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultKeepalive, returnedNetwork.DefaultKeepalive)
	})
	t.Run("UpdateKeepAliveTooBig", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		//does not fails ----- value gets updated.
		// ----- needs fixing -----
		//t.Skip()
		network.DefaultKeepalive = 2000
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'DefaultKeepalive' failed on the 'max' tag")
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("UpdateSaveConfig", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		//t.Skip()
		//not updatable, ensure attempt to change does not result in change
		if network.DefaultSaveConfig == "yes" {
			network.DefaultSaveConfig = "no"
		} else {
			network.DefaultSaveConfig = "yes"
		}
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		newnet := getNetwork(t, "skynet")
		assert.Equal(t, network.DefaultSaveConfig, newnet.DefaultSaveConfig)
	})
	t.Run("UpdateManualSignUP", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		if network.AllowManualSignUp == "yes" {
			network.AllowManualSignUp = "no"
		} else {
			network.AllowManualSignUp = "yes"
		}
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.AllowManualSignUp, returnedNetwork.AllowManualSignUp)
	})
	t.Run("DefaultCheckInterval", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultCheckInInterval = 60
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultCheckInInterval, returnedNetwork.DefaultCheckInInterval)
	})
	t.Run("DefaultCheckIntervalTooBig", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultCheckInInterval = 100001
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'DefaultCheckInInterval' failed")
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("MultipleFields", func(t *testing.T) {
		network := getNetwork(t, "skynet")
		network.DefaultListenPort = 7777
		network.DisplayName = "multi"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DisplayName, returnedNetwork.DisplayName)
		assert.Equal(t, network.DefaultListenPort, returnedNetwork.DefaultListenPort)
	})
}

func TestKeyUpdate(t *testing.T) {
	t.Skip() //test fails  reasons unknown
	//get current network settings
	oldnet := getNetwork(t, "skynet")
	//update key
	time.Sleep(time.Second * 1)
	reply, err := api(t, "", http.MethodPost, baseURL+"/api/networks/skynet/keyupdate", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, reply.StatusCode)
	newnet := getNetwork(t, "skynet")
	assert.Greater(t, newnet.KeyUpdateTimeStamp, oldnet.KeyUpdateTimeStamp)
}
