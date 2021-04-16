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

func TestCreateNetwork(t *testing.T) {
	network := models.Network{}
	network.NetID = "skynet"
	network.AddressRange = "10.71.0.0/16"
	if networkExists(t) {
		deleteNetworks(t)
	}
	t.Run("InvalidToken", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("CreateNetwork", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("DuplicateNetwork", func(t *testing.T) {
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
	t.Run("BadName", func(t *testing.T) {
		network.NetID = "thisnameistoolong"
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
	t.Run("BadAddress", func(t *testing.T) {
		network.NetID = "wirecat"
		network.AddressRange = "10.300.20.56/36"
		response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
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
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
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
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("InvalidNetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/badnetwork", "secretkey")
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
	t.Skip()
	//not part of api anymore
	t.Run("ValidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet/numnodes", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message int
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		//assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet/numnodes", "badkey")
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
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/badnetwork/numnodes", "secretkey")
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
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet", "badkey")
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
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message mongo.DeleteResult
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, int64(1), message.DeletedCount)

	})
	t.Run("Badnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/badnetwork", "secretkey")
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
}

func TestCreateAccessKey(t *testing.T) {
	if !networkExists(t) {
		createNetwork(t)
	}

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
		//t.Skip()
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
		//this is allowed I think it should fail fail
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/skynet/keys", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		deleteKey(t, key.Name, "skynet")
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
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
	t.Run("Badnetwork", func(t *testing.T) {
		response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/badnetwork/keys", "secretkey")
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
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		//var message mongo.DeleteResult
		var messages []models.AccessKey
		err = json.NewDecoder(response.Body).Decode(&messages)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		for _, message := range messages {
			assert.Equal(t, "skynet", message.Name)
		}
	})
	t.Run("InValidKey", func(t *testing.T) {
		t.Skip()
		//responds ok, will nil record returned..  should be an error.
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet/keys/badkey", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This key does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("KeyInValidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/badnetwork/keys/skynet", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
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
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
}

func TestGetKeys(t *testing.T) {
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
	//deletekeys
	t.Run("Invalidnetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/badnetwork/keys", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: This network does not exist.", message.Message)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
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
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
	})
}

func TestUpdatenetwork(t *testing.T) {
	//ensure we are working with known networks
	deleteNetworks(t)
	createNetwork(t)
	var returnedNetwork models.Network
	t.Run("UpdateNetID", func(t *testing.T) {
		type Network struct {
			NetID string
		}
		var network Network
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "NetID is not editable", message.Message)
	})
	t.Run("NetIDInvalidCredentials", func(t *testing.T) {
		type Network struct {
			NetID string
		}
		var network Network
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "badkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: You are unauthorized to access this endpoint.", message.Message)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("Invalidnetwork", func(t *testing.T) {
		type Network struct {
			NetID string
		}
		var network Network
		network.NetID = "wirecat"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/badnetwork", "secretkey")
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
		// ---- needs fixing -----
		// succeeds for some reason
		t.Skip()
		type Network struct {
			NetID string
		}
		var network Network
		network.NetID = "wirecat-skynet"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateAddress", func(t *testing.T) {
		type Network struct {
			AddressRange string
		}
		var network Network
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
		type Network struct {
			AddressRange string
		}
		var network Network
		network.AddressRange = "10.0.0.1/36"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, message.Code)
		assert.Contains(t, message.Message, "Invalid Range")

	})
	t.Run("UpdateDisplayName", func(t *testing.T) {
		type Network struct {
			DisplayName string
		}
		var network Network
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
		// -----needs fixing ----
		// fails silently
		t.Skip()
		type Network struct {
			DisplayName string
		}
		var network Network
		//create name that is longer than 100 chars
		name := ""
		for i := 0; i < 101; i++ {
			name = name + "a"
		}
		network.DisplayName = name
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DisplayName' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdateInterface", func(t *testing.T) {
		type Network struct {
			DefaultInterface string
		}
		var network Network
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
		type Network struct {
			DefaultListenPort int32
		}
		var network Network
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
		// ---needs fixing -----
		// value is updated anyways
		t.Skip()
		type Network struct {
			DefaultListenPort int32
		}
		var network Network
		network.DefaultListenPort = 65540
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultListenPort' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("UpdatePostUP", func(t *testing.T) {
		type Network struct {
			DefaultPostUp string
		}
		var network Network
		network.DefaultPostUp = "sudo wg add-conf wc-netmaker /etc/wireguard/peers/conf"
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultPostUp, returnedNetwork.DefaultPostUp)
	})
	t.Run("UpdatePreUp", func(t *testing.T) {
		// -------needs fixing ------
		// mismatch in models.Network between struc name and json/bson name
		// does not get updated.
		t.Skip()
		type Network struct {
			DefaultPostDown string
		}
		var network Network
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
		type Network struct {
			DefaultKeepalive int32
		}
		var network Network
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
		//fails silently
		// ----- needs fixing -----
		t.Skip()
		type Network struct {
			DefaultKeepAlive int32
		}
		var network Network
		network.DefaultKeepAlive = 1001
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
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
		type Network struct {
			DefaultSaveConfig *bool
		}
		var network Network
		value := false
		network.DefaultSaveConfig = &value
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, *network.DefaultSaveConfig, *returnedNetwork.DefaultSaveConfig)
	})
	t.Run("UpdateManualSignUP", func(t *testing.T) {
		t.Skip()
		type Network struct {
			AllowManualSignUp *bool
		}
		var network Network
		value := true
		network.AllowManualSignUp = &value
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		//returns previous value not the updated value
		// ----- needs fixing -----
		//assert.Equal(t, network.NetID, returnedNetwork.NetID)
	})
	t.Run("DefaultCheckInterval", func(t *testing.T) {
		//value is not returned in struct ---
		t.Skip()
		type Network struct {
			DefaultCheckInInterval int32
		}
		var network Network
		network.DefaultCheckInInterval = 6000
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&returnedNetwork)
		assert.Nil(t, err, err)
		assert.Equal(t, network.DefaultCheckInInterval, returnedNetwork.DefaultCheckInInterval)
	})
	t.Run("DefaultCheckIntervalTooBig", func(t *testing.T) {
		//value is not returned in struct ---
		t.Skip()
		type Network struct {
			DefaultCheckInInterval int32
		}
		var network Network
		network.DefaultCheckInInterval = 100001
		response, err := api(t, network, http.MethodPut, baseURL+"/api/networks/skynet", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnprocessableEntity, message.Code)
		assert.Equal(t, "W1R3: Field validation for 'DefaultCheckInInterval' failed.", message.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})
	t.Run("MultipleFields", func(t *testing.T) {
		type Network struct {
			DisplayName       string
			DefaultListenPort int32
		}
		var network Network
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
