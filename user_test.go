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

type AuthorizationResponse struct {
	Username  string
	AuthToken string
}

type goodResponse struct {
	Code     int
	Message  string
	Response AuthorizationResponse
}

type badResponse struct {
	Code    int
	Message string
}

//assumption:  starting with empty database
func TestMain(m *testing.M) {
	mongoconn.ConnectDatabase()
	var waitgroup sync.WaitGroup
	waitgroup.Add(1)
	go controller.HandleRESTRequests(&waitgroup)
	//wait for http server to start
	time.Sleep(time.Second * 1)
	os.Exit(m.Run())
}

func TestUsers(t *testing.T) {

	t.Run("check that admin user does not exist", func(t *testing.T) {
		response := checkAdminExists(t)
		assert.Equal(t, false, response)
	})

	t.Run("add admin user", func(t *testing.T) {
		var admin, user models.User
		admin.UserName = "admin"
		admin.Password = "password"

		//payload := map[string]string{"username": "admin", "password": "admin"}
		payload, _ := json.Marshal(admin)
		request, err := http.NewRequest(http.MethodPost, "http://localhost:8081/users/createadmin", bytes.NewBuffer(payload))
		if err != nil {
			t.Error(err)
		}
		request.Header.Set("Authorization", "Bearer secretkey")
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			t.Error("error calling createadmin", err)
		}
		defer response.Body.Close()
		body, _ := ioutil.ReadAll(response.Body)
		_ = json.Unmarshal(body, &user)
		assert.Equal(t, admin.UserName, user.UserName)
		assert.Equal(t, true, user.IsAdmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		adminExists := checkAdminExists(t)
		assert.Equal(t, true, adminExists)
	})

	t.Run("GetUser", func(t *testing.T) {
		t.Skip()
		//ensure admin exists
		if !checkAdminExists(t) {
			t.Error("admin account does not exist")
			return
		}
		//authenticate
		var admin models.User
		admin.UserName = "admin"
		admin.Password = "admin"

		payload, _ := json.Marshal(admin)
		request, err := http.NewRequest(http.MethodPut, "http://localhost:8081/users/authenticate", bytes.NewBuffer(payload))
		if err != nil {
			t.Error(err)
		}
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}

		response, err := client.Do(request)
		if err != nil {
			t.Error("error calling authenticate", err)
		}
		defer response.Body.Close()
		body := models.User{}
		json.NewDecoder(response.Body).Decode(&body)
		t.Log(body)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "admin", body.UserName)

		request, err = http.NewRequest(http.MethodGet, "http://localhost:8081/users/admim", nil)
		if err != nil {
			t.Error(err)
		}
		request.Header.Set("Authorization", "Bearer secretkey")
		client = &http.Client{}
		response, err = client.Do(request)
		if err != nil {
			t.Error(err)
		}
		defer response.Body.Close()
		body = models.User{}
		json.NewDecoder(response.Body).Decode(&body)
		t.Log(body)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "admin", body.UserName)
	})

	t.Run("Update User", func(t *testing.T) {
		t.Skip()
		var admin, user models.User
		admin.UserName = "admin"
		admin.Password = "admin"

		//payload := map[string]string{"username": "admin", "password": "admin"}
		payload, _ := json.Marshal(admin)
		request, err := http.NewRequest(http.MethodPut, "http://localhost:8081/users/admin", bytes.NewBuffer(payload))
		if err != nil {
			t.Error(err)
		}
		request.Header.Set("Authorization", "Bearer secretkey")
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			t.Error("error calling createadmin", err)
		}
		defer response.Body.Close()
		body, _ := ioutil.ReadAll(response.Body)
		t.Log(string(body))
		_ = json.Unmarshal(body, &user)
		assert.Equal(t, admin.UserName, user.UserName)
		assert.Equal(t, true, user.IsAdmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("Authenticate User", func(t *testing.T) {
		var admin models.User

		admin.UserName = "admin"
		admin.Password = "password"

		payload, _ := json.Marshal(admin)
		request, err := http.NewRequest(http.MethodPost, "http://localhost:8081/users/authenticate", bytes.NewBuffer(payload))
		if err != nil {
			t.Error(err)
		}
		request.Header.Set("Authorization", "Bearer secretkey")
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			t.Error(err)
		}
		defer response.Body.Close()
		message := goodResponse{}
		json.NewDecoder(response.Body).Decode(&message)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "W1R3: Device admin Authorized", message.Message)
	})

	t.Run("empty test", func(t *testing.T) {
		assert.Equal(t, true, true)
	})

}

func checkAdminExists(t *testing.T) bool {
	response, err := http.Get("http://localhost:8081/users/hasadmin")
	if err != nil {
		t.Fatal("error calling users/hasadmin", err)
	}
	defer response.Body.Close()
	var body bool
	json.NewDecoder(response.Body).Decode(&body)
	return body
	//body, _ := ioutil.ReadAll(response.Body)
	//vif body {
	//	return true
	//}
	//return false
}
