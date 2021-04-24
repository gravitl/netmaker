package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestRainyDay(t *testing.T) {
	t.Run("badkey", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes", "badkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("badURL", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/adm/skynet/01:02:03:04:05:07", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
	t.Run("NonExistentNetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/badnet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestGetAllNodes(t *testing.T) {
	setup(t)
	t.Run("NodesExist", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var nodes []models.ReturnNode
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&nodes)
		assert.Nil(t, err, err)
		assert.NotEmpty(t, nodes)
	})
	t.Run("NodeDoesNotExist", func(t *testing.T) {
		deleteNode(t)
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var nodes []models.ReturnNode
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&nodes)
		assert.Nil(t, err, err)
		assert.Empty(t, nodes)
	})
}

func TestGetNetworkNodes(t *testing.T) {
	setup(t)
	t.Run("NodeExists", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var nodes []models.ReturnNode
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&nodes)
		assert.Nil(t, err, err)
		assert.NotEmpty(t, nodes)
	})
	t.Run("NodeDoesNotExit", func(t *testing.T) {
		deleteNode(t)
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var nodes []models.ReturnNode
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&nodes)
		assert.Nil(t, err, err)
		assert.Empty(t, nodes)
	})
}

func TestGetNode(t *testing.T) {
	setup(t)
	t.Run("NodeExists", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var node models.Node
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&node)
		assert.Nil(t, err, err)
		assert.Equal(t, "01:02:03:04:05:06", node.MacAddress)
	})
	t.Run("NodeDoesNotExist", func(t *testing.T) {
		deleteNode(t)
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
}

func TestUpdateNode(t *testing.T) {
	var data models.Node
	setup(t)

	t.Run("UpdateMulti", func(t *testing.T) {
		data.MacAddress = "01:02:03:04:05:05"
		data.Name = "NewName"
		data.PublicKey = "DM5qhLAE20PG9BbfBCgfr+Ac9D2NDOwCtY1rbYDLf34="
		data.Password = "newpass"
		data.LocalAddress = "192.168.0.2"
		data.Endpoint = "10.100.100.5"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var node models.Node
		t.Log(response.Body)
		err = json.NewDecoder(response.Body).Decode(&node)
		assert.Nil(t, err, err)
		assert.Equal(t, data.Name, node.Name)
		assert.Equal(t, data.PublicKey, node.PublicKey)
		err = bcrypt.CompareHashAndPassword([]byte(node.Password), []byte(data.Password))
		assert.Nil(t, err, err)
		assert.Equal(t, data.LocalAddress, node.LocalAddress)
		assert.Equal(t, data.Endpoint, node.Endpoint)
	})
	t.Run("InvalidMacAddress", func(t *testing.T) {
		data.MacAddress = "10:11:12:13:14:15:16"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'MacAddress' failed")
	})
	t.Run("InvalidEndpoint", func(t *testing.T) {
		data.Endpoint = "10.10.10.300"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'Endpoint' failed")
	})
	t.Run("InvalidLocalAddress", func(t *testing.T) {
		data.LocalAddress = "10.10.10.300"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'LocalAddress' failed")
	})
	t.Run("InvalidName", func(t *testing.T) {
		var data struct {
			Name string
		}
		data.Name = "New*Name"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'Name' failed")
	})
	t.Run("InvalidPublicKey", func(t *testing.T) {
		data.PublicKey = "xxx"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'PublicKey' failed")
	})
	t.Run("InvalidPassword", func(t *testing.T) {
		data.Password = "1234"
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'Password' failed")
	})
	t.Run("EmptyPassword", func(t *testing.T) {
		data.Password = ""
		response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:05", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'Password' failed")
	})
}

func TestDeleteNode(t *testing.T) {
	setup(t)
	t.Run("ExistingNode", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var message models.SuccessResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "01:02:03:04:05:06 deleted.", message.Message)
		assert.Equal(t, http.StatusOK, message.Code)
		t.Log(response.Header.Get("Content-Type"))
	})
	t.Run("NonExistantNode", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, message.Code)
		assert.Equal(t, "Could not delete node 01:02:03:04:05:06", message.Message)
	})
}

func TestCheckIn(t *testing.T) {
	setup(t)
	oldNode := getNode(t)
	time.Sleep(1 * time.Second)
	t.Run("Valid", func(t *testing.T) {
		response, err := api(t, "", http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/checkin", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		var node models.Node
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&node)
		assert.Nil(t, err, err)
		assert.Greater(t, node.LastCheckIn, oldNode.LastCheckIn)
	})
	t.Run("NodeDoesNotExist", func(t *testing.T) {
		deleteNode(t)
		response, err := api(t, "", http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/checkin", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		var message models.ErrorResponse
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, message.Code)
		assert.Equal(t, "mongo: no documents in result", message.Message)
	})
}

func TestCreateGateway(t *testing.T) {
	setup(t)
	//assert.False(t, node.IsGateway)
	var gateway models.GatewayRequest
	t.Run("Valid", func(t *testing.T) {
		gateway.RangeString = "0.0.0.0/0"
		gateway.Interface = "eth0"
		response, err := api(t, gateway, http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/creategateway", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var message models.Node
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.True(t, message.IsGateway)
	})
	t.Run("BadRange", func(t *testing.T) {
		gateway.RangeString = "0.0.0.0/36"
		gateway.Interface = "eth0"
		response, err := api(t, gateway, http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/creategateway", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, message.Code)
		assert.Equal(t, "IP Range Not Valid", message.Message)
	})
	t.Run("BadInterface", func(t *testing.T) {
		gateway.RangeString = "0.0.0.0/0"
		gateway.Interface = ""
		response, err := api(t, gateway, http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/creategateway", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusInternalServerError, message.Code)
		assert.Equal(t, "Interface cannot be empty", message.Message)
	})
}

func TestDeleteGateway(t *testing.T) {
	setup(t)
	response, err := api(t, "", http.MethodDelete, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/deletegateway", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var message models.Node
	err = json.NewDecoder(response.Body).Decode(&message)
	assert.Nil(t, err, err)
	assert.False(t, message.IsGateway)
}

func TestUncordonNode(t *testing.T) {
	setup(t)
	response, err := api(t, "", http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/approve", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var message string
	err = json.NewDecoder(response.Body).Decode(&message)
	assert.Nil(t, err, err)
	assert.Equal(t, "SUCCESS", message)
	t.Log(message, string(message))
}

func TestCreateNode(t *testing.T) {
	setup(t)
	key := createAccessKey(t)
	t.Run("NodeExists", func(t *testing.T) {
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Contains(t, message.Message, "Field validation for 'MacAddress' failed on the 'macaddress_unique' tag")
	})
	t.Run("BadKey", func(t *testing.T) {
		deleteNode(t)
		var node models.Node
		node.AccessKey = "badkey"
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Contains(t, message.Message, "W1R3: Key invalid, or none provided")
	})
	t.Run("BadMac", func(t *testing.T) {
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "badmac"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'MacAddress' failed on the 'macaddress_valid' tag")
	})
	t.Run("BadPublicKey", func(t *testing.T) {
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "xxx"
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'PublicKey' failed")
	})
	t.Run("BadPass", func(t *testing.T) {
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = ""
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Error:Field validation for 'Password' failed")
	})
	t.Run("BadLocalAddress", func(t *testing.T) {
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.300.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'LocalAddress' failed")
	})
	t.Run("BadEndpoint", func(t *testing.T) {
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.300.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Contains(t, message.Message, "Field validation for 'Endpoint' failed")
	})
	t.Run("NetworkDoesNotExist", func(t *testing.T) {
		deleteNetworks(t)
		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, message.Code)
		assert.Contains(t, "W1R3: Network does not exist! ", message.Message)
	})
	t.Run("Valid", func(t *testing.T) {
		setup(t)
		deleteNode(t)
		key := createAccessKey(t)

		var node models.Node
		node.AccessKey = key.Value
		node.MacAddress = "01:02:03:04:05:06"
		node.Name = "myNode"
		node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
		node.Password = "tobedetermined"
		node.LocalAddress = "192.168.0.1"
		node.Endpoint = "10.100.100.4"

		response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var message models.Node
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, node.Name, message.Name)
		t.Log(message)
	})

}

func TestGetLastModified(t *testing.T) {
	deleteNetworks(t)
	createNetwork(t)
	t.Run("Valid", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/adm/skynet/lastmodified", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	deleteNetworks(t)
	t.Run("NoNetwork", func(t *testing.T) {
		response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/adm/skynet/lastmodified", "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestNodeAuthenticate(t *testing.T) {
	setup(t)
	t.Run("Valid", func(t *testing.T) {
		var authRequest models.AuthParams
		authRequest.MacAddress = "01:02:03:04:05:06"
		authRequest.Password = "tobedetermined"
		response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		defer response.Body.Close()
		var message models.SuccessResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, message.Code)
		assert.Equal(t, "W1R3: Device 01:02:03:04:05:06 Authorized", message.Message)
	})
	t.Run("MacEmpty", func(t *testing.T) {
		var authRequest models.AuthParams
		authRequest.MacAddress = ""
		authRequest.Password = "tobedetermined"
		response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.SuccessResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "W1R3: MacAddress can't be empty", message.Message)
	})
	t.Run("EmptyPass", func(t *testing.T) {
		var authRequest models.AuthParams
		authRequest.MacAddress = "01:02:03:04:05:06"
		authRequest.Password = ""
		response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.SuccessResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "W1R3: Password can't be empty", message.Message)
	})
	t.Run("BadPass", func(t *testing.T) {
		var authRequest models.AuthParams
		authRequest.MacAddress = "01:02:03:04:05:06"
		authRequest.Password = "badpass"
		response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.SuccessResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "crypto/bcrypt: hashedPassword is not the hash of the given password", message.Message)
	})
	t.Run("BadMac", func(t *testing.T) {
		var authRequest models.AuthParams
		authRequest.MacAddress = "01:02:03:04:05:07"
		authRequest.Password = "tobedetermined"
		response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		defer response.Body.Close()
		var message models.SuccessResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusBadRequest, message.Code)
		assert.Equal(t, "mongo: no documents in result", message.Message)
	})
}
