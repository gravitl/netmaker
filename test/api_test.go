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
	"github.com/gravitl/netmaker/mongoconn"
	"github.com/stretchr/testify/assert"
)

type databaseError struct {
	Inner  *int
	Errors int
}

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

func TestMain(m *testing.M) {
	mongoconn.ConnectDatabase()
	var waitgroup sync.WaitGroup
	waitgroup.Add(1)
	go controller.HandleRESTRequests(&waitgroup)
	//wait for http server to start
	time.Sleep(time.Second * 1)
	os.Exit(m.Run())
}

func adminExists(t *testing.T) bool {
	response, err := http.Get("http://localhost:8081/users/hasadmin")
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
		request.Header.Set("Authorization", "Bearer "+authorization)
	}
	client := http.Client{}
	return client.Do(request)
}

func addAdmin(t *testing.T) {
	var admin models.User
	admin.UserName = "admin"
	admin.Password = "password"
	response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func authenticate(t *testing.T) (string, error) {
	var admin models.User
	admin.UserName = "admin"
	admin.Password = "password"
	response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/authenticate", "secretkey")
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
	_, err = api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", token)
	assert.Nil(t, err, err)
}

func createGroup(t *testing.T) {
	group := models.Group{}
	group.NameID = "skynet"
	group.AddressRange = "10.71.0.0/16"
	response, err := api(t, group, http.MethodPost, "http://localhost:8081/api/groups", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func createKey(t *testing.T) {
	key := models.AccessKey{}
	key.Name = "skynet"
	key.Uses = 10
	response, err := api(t, key, http.MethodPost, "http://localhost:8081/api/groups/skynet/keys", "secretkey")
	assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	message, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err, err)
	assert.NotNil(t, message, message)
}

func getKey(t *testing.T, name string) models.AccessKey {
	response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups/skynet/keys", "secretkey")
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

func deleteKey(t *testing.T, key, group string) {
	response, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/"+group+"/keys/"+key, "secretkey")
	assert.Nil(t, err, err)
	//api does not return Deleted Count at this time
	//defer response.Body.Close()
	//var message mongo.DeleteResult
	//err = json.NewDecoder(response.Body).Decode(&message)
	//assert.Nil(t, err, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	//assert.Equal(t, int64(1), message.DeletedCount)
}

func groupExists(t *testing.T) bool {
	response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups", "secretkey")
	assert.Nil(t, err, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	err = json.NewDecoder(response.Body).Decode(&Groups)
	assert.Nil(t, err, err)
	if Groups == nil {
		return false
	} else {
		return true
	}
}

func deleteGroups(t *testing.T) {

	response, err := api(t, "", http.MethodGet, "http://localhost:8081/api/groups", "secretkey")
	assert.Nil(t, err, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	err = json.NewDecoder(response.Body).Decode(&Groups)
	assert.Nil(t, err, err)
	for _, group := range Groups {
		name := group.DisplayName
		_, err := api(t, "", http.MethodDelete, "http://localhost:8081/api/groups/"+name, "secretkey")
		assert.Nil(t, err, err)
	}
}
