package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestGetAllNodes(t *testing.T) {
	//ensure nodes exist
	response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var nodes []models.ReturnNode
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&nodes)
	assert.Nil(t, err, err)
	for _, node := range nodes {
		assert.NotNil(t, node, "empty node")
	}
}

func TestGetNetworkNodes(t *testing.T) {
	response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var nodes []models.ReturnNode
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&nodes)
	assert.Nil(t, err, err)
	for _, node := range nodes {
		assert.NotNil(t, node, "empty node")
	}
}

func TestGetNode(t *testing.T) {
	response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var node models.Node
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&node)
	assert.Nil(t, err, err)
	assert.Equal(t, "01:02:03:04:05:06", node.MacAddress)
}

func TestUpdateNode(t *testing.T) {
	var data struct {
		Name string
	}
	data.Name = "NewName"
	response, err := api(t, data, http.MethodPut, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var node models.Node
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&node)
	assert.Nil(t, err, err)
	assert.Equal(t, data.Name, node.Name)
}

func TestDeleteNode(t *testing.T) {
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
	//get node
	//oldNode := getNode(t)
	response, err := api(t, "", http.MethodPost, baseURL+"/api/nodes/skynet/01:02:03:04:05:06/checkin", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var node models.Node
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&node)
	assert.Nil(t, err, err)
	//assert.Greater(t, node.LastCheckIn, oldNode.LastCheckin)
}

func TestCreateGateway(t *testing.T) {
	//assert.False(t, node.IsGateway)
	var gateway models.GatewayRequest
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
}

func TestDeleteGateway(t *testing.T) {
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
	//setup environment
	nodes := getNetworkNodes(t)
	for _, node := range nodes {
		deleteNode(t, node)
	}
	deleteNetworks(t)
	createNetwork(t)
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
	//message, err := ioutil.ReadAll(response.Body)
	//assert.Nil(t, err, err)
	var message models.Node
	err = json.NewDecoder(response.Body).Decode(&message)
	assert.Nil(t, err, err)
	assert.Equal(t, node.Name, message.Name)
	//nodePassword = message.Password
	t.Log(message.Password)
}

func TestGetLastModified(t *testing.T) {
	response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/adm/skynet/lastmodified", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.NotNil(t, response.Body, "no time returned")
}

func TestNodeAuthenticate(t *testing.T) {
	//setup
	//deleteA
	deleteNetworks(t)
	createNetwork(t)
	//password := createNode(t)
	var authRequest models.AuthParams
	authRequest.MacAddress = "01:02:03:04:05:06"
	//authRequest.MacAddress = "mastermac"
	//authRequest.Password = nodePassword
	authRequest.Password = "secretkey"
	response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	message, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err, err)
	//var message string
	//json.NewDecoder(response.Body).Decode(&message)
	t.Log(string(message))

}

func TestNodeAuthorize(t *testing.T) {
	//testing
	var authRequest models.AuthParams
	//authRequest.MacAddress = "01:02:03:04:05:06"
	authRequest.MacAddress = "mastermac"
	authRequest.Password = "to be determined"
	response, err := api(t, authRequest, http.MethodPost, "http://localhost:8081:/api/nodes/adm/skynet/authenticate", "")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var message string
	json.NewDecoder(response.Body).Decode(&message)
	t.Log(message)
}
