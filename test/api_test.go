package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/models"
	""
	"github.com/stretchr/testify/assert"
)

//should be use models.SuccessResponse and models.SuccessfulUserLoginResponse
//rather than creating new type but having trouble decoding that way
type Auth struct {
	Username  string
	AuthToken string
}
type Success struct {
	Code     int
	Message  string
	Response Auth
}

type AuthorizeTestCase struct {
	testname      string
	name          string
	password      string
	code          int
	tokenExpected bool
	errMessage    string
}

var Networks []models.Network
var baseURL string = "http://localhost:8081"

func TestMain(m *testing.M) {
	mongoconn.ConnectDatabase()
	var waitgroup sync.WaitGroup
	waitgroup.Add(1)
	go controller.HandleRESTRequests(&waitgroup)
	time.Sleep(time.Second * 1)
	os.Exit(m.Run())
}

func adminExists(t *testing.T) bool {
	response, err := http.Get("http://localhost:8081/api/users/adm/hasadmin")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var body bool
	json.NewDecoder(response.Body).Decode(&body)
	return body
}

func api(t *testing.T, data interface{}, method, url, authorization string) (*http.Response, error) {
	var request *http.Request
	var err error
	if data != "" {
		payload, err := json.Marshal(data)
		assert.Nil(t, err, err)
		request, err = http.NewRequest(method, url, bytes.NewBuffer(payload))
		assert.Nil(t, err, err)
		request.Header.Set("Content-Type", "application/json")
	} else {
		request, err = http.NewRequest(method, url, nil)
		assert.Nil(t, err, err)
	}
	if authorization != "" {
		request.Header.Set("authorization", "Bearer "+authorization)
	}
	client := http.Client{}
	return client.Do(request)
}

func addAdmin(t *testing.T) {
	var admin models.UserAuthParams
	admin.UserName = "admin"
	admin.Password = "password"
	response, err := api(t, admin, http.MethodPost, baseURL+"/api/users/adm/createadmin", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func authenticate(t *testing.T) (string, error) {
	var admin models.User
	admin.UserName = "admin"
	admin.Password = "password"
	response, err := api(t, admin, http.MethodPost, baseURL+"/api/users/adm/authenticate", "secretkey")
	assert.Nil(t, err, err)

	var body Success
	err = json.NewDecoder(response.Body).Decode(&body)
	assert.Nil(t, err, err)
	assert.NotEmpty(t, body.Response.AuthToken, "token not returned")
	assert.Equal(t, "W1R3: Device admin Authorized", body.Message)

	return body.Response.AuthToken, nil
}

func deleteAdmin(t *testing.T) {
	if !adminExists(t) {
		return
	}
	token, err := authenticate(t)
	assert.Nil(t, err, err)
	_, err = api(t, "", http.MethodDelete, baseURL+"/api/users/admin", token)
	assert.Nil(t, err, err)
}

func createNetwork(t *testing.T) {
	network := models.Network{}
	network.NetID = "skynet"
	network.AddressRange = "10.71.0.0/16"
	response, err := api(t, network, http.MethodPost, baseURL+"/api/networks", "secretkey")
	assert.Nil(t, err, err)
	t.Log(err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func createKey(t *testing.T) {
	key := models.AccessKey{}
	key.Name = "skynet"
	key.Uses = 10
	response, err := api(t, key, http.MethodPost, baseURL+"/api/networks/skynet/keys", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	message, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err, err)
	assert.NotNil(t, message, message)
}

func createAccessKey(t *testing.T) (key models.AccessKey) {
	//delete existing key if
	_, _ = api(t, "", http.MethodDelete, baseURL+"/api/networks/skynet/keys/skynet", "secretkey")
	createkey := models.AccessKey{}
	createkey.Name = "skynet"
	createkey.Uses = 10
	response, err := api(t, createkey, http.MethodPost, baseURL+"/api/networks/skynet/keys", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&key)
	assert.Nil(t, err, err)
	return key
}

func getKey(t *testing.T, name string) models.AccessKey {
	response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/skynet/keys", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var keys []models.AccessKey
	err = json.NewDecoder(response.Body).Decode(&keys)
	assert.Nil(t, err, err)
	for _, key := range keys {
		if key.Name == name {
			return key
		}
	}
	return models.AccessKey{}
}

func deleteKey(t *testing.T, key, network string) {
	response, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/"+network+"/keys/"+key, "secretkey")
	assert.Nil(t, err, err)
	//api does not return Deleted Count at this time
	//defer response.Body.Close()
	//var message mongo.DeleteResult
	//err = json.NewDecoder(response.Body).Decode(&message)
	//assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	//assert.Equal(t, int64(1), message.DeletedCount)
}

func deleteNetworks(t *testing.T) {
	//delete all node
	deleteAllNodes(t)
	response, err := api(t, "", http.MethodGet, baseURL+"/api/networks", "secretkey")
	assert.Nil(t, err, err)
	if response.StatusCode == http.StatusOK {
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&Networks)
		assert.Nil(t, err, err)
		for _, network := range Networks {
			name := network.NetID
			_, err := api(t, "", http.MethodDelete, baseURL+"/api/networks/"+name, "secretkey")
			assert.Nil(t, err, err)
		}
	}
}

func deleteNode(t *testing.T) {
	response, err := api(t, "", http.MethodDelete, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}
func deleteAllNodes(t *testing.T) {
	response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var nodes []models.Node
	defer response.Body.Close()
	json.NewDecoder(response.Body).Decode(&nodes)
	for _, node := range nodes {
		resp, err := api(t, "", http.MethodDelete, baseURL+"/api/nodes/"+node.Network+"/"+node.MacAddress, "secretkey")
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}
func createNode(t *testing.T) {
	var node models.Node
	key := createAccessKey(t)
	node.Address = "10.71.0.1"
	node.AccessKey = key.Value
	node.MacAddress = "01:02:03:04:05:06"
	node.Name = "myNode"
	node.PublicKey = "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
	node.Password = "tobedetermined"
	node.Endpoint = "10.100.100.4"
	response, err := api(t, node, http.MethodPost, "http://localhost:8081:/api/nodes/skynet", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func getNode(t *testing.T) models.Node {
	response, err := api(t, "", http.MethodGet, baseURL+"/api/nodes/skynet/01:02:03:04:05:06", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var node models.Node
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&node)
	assert.Nil(t, err, err)
	return node
}

func getNetwork(t *testing.T, network string) models.Network {
	var net models.Network
	response, err := api(t, "", http.MethodGet, baseURL+"/api/networks/"+network, "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&net)
	assert.Nil(t, err, err)
	return net
}

func setup(t *testing.T) {
	deleteNetworks(t)
	createNetwork(t)
	createNode(t)
}
