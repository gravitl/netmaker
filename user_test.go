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

//change this --- there is an existing type
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

func TestUsers(t *testing.T) {

	//	t.Run("check that admin user does not exist", func(t *testing.T) {
	//		response := checkAdminExists(t)
	//		assert.Equal(t, false, response)
	//	})

	t.Run("Admin Creation", func(t *testing.T) {
		var admin, user models.User
		admin.UserName = "admin"
		admin.Password = "password"
		if !adminExists(t) {
			t.Run("AdminCreationValid", func(t *testing.T) {
				response, err := api(admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
				if err != nil {
					t.Error("error calling createadmin", err)
				}
				defer response.Body.Close()
				json.NewDecoder(response.Body).Decode(&user)
				assert.Equal(t, admin.UserName, user.UserName)
				assert.Equal(t, true, user.IsAdmin)
				assert.Equal(t, http.StatusOK, response.StatusCode)
				assert.True(t, adminExists(t), "Admin creation failed")
			})
			t.Run("AdminCreationInvalid", func(t *testing.T) {
				response, err := api(admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
				if err != nil {
					t.Error("error calling createadmin", err)
				}
				defer response.Body.Close()
				var message badResponse
				json.NewDecoder(response.Body).Decode(&message)
				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Equal(t, http.StatusUnauthorized, message.Code)
				assert.Equal(t, "W1R3: Admin already exists! ", message.Message)
			})
		} else {
			t.Run("AdminCreationInvalid", func(t *testing.T) {
				response, err := api(admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")

				if err != nil {
					t.Error("error calling createadmin", err)
				}
				defer response.Body.Close()
				var message badResponse
				json.NewDecoder(response.Body).Decode(&message)
				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Equal(t, http.StatusUnauthorized, message.Code)
				assert.Equal(t, "W1R3: Admin already exists! ", message.Message)
			})
			//deleteAdmin()
			t.Run("Admin Creation - Valid", func(t *testing.T) {
				t.Skip()
				response, err := api(admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
				if err != nil {
					t.Error("error calling createadmin", err)
				}
				defer response.Body.Close()
				json.NewDecoder(response.Body).Decode(&user)
				assert.Equal(t, admin.UserName, user.UserName)
				assert.Equal(t, true, user.IsAdmin)
				assert.Equal(t, http.StatusOK, response.StatusCode)
				assert.True(t, adminExists(t), "Admin creation failed")
			})
		}
	})

	t.Run("GetUser", func(t *testing.T) {
		//ensure admin exists
		if !adminExists(t) {
			t.Error("admin account does not exist")
			return
		}
		//authenticate
		token, err := authenticate()
		if err != nil {
			t.Error("could not authenticate")
		}
		response, err := api("", http.MethodGet, "http://localhost:8081/users/admin", token)
		if err != nil {
			t.Error("could not get user")
		}
		defer response.Body.Close()
		var user models.User
		json.NewDecoder(response.Body).Decode(&user)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "admin", user.UserName)
		assert.Equal(t, true, user.IsAdmin)

	})

	t.Run("Update User", func(t *testing.T) {
		t.Skip()
		if !adminExists(t) {
			addAdmin(t)
		}
		//token, err := authenticate()
		//if err != nil {
		//	t.Error("could not authenticate")
		//}

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
		cases := []AuthorizeTestCase{
			AuthorizeTestCase{
				testname:      "Invalid User",
				name:          "invaliduser",
				password:      "password",
				code:          http.StatusBadRequest,
				tokenExpected: false,
				errMessage:    "W1R3: It's not you it's me.",
			},
			AuthorizeTestCase{
				testname:      "empty user",
				name:          "",
				password:      "password",
				code:          http.StatusBadRequest,
				tokenExpected: false,
				errMessage:    "W1R3: Username can't be empty",
			},
			AuthorizeTestCase{
				testname:      "empty password",
				name:          "admin",
				password:      "",
				code:          http.StatusBadRequest,
				tokenExpected: false,
				errMessage:    "W1R3: Password can't be empty",
			},
			AuthorizeTestCase{
				testname:      "Invalid Passord",
				name:          "admin",
				password:      "xxxxxxx",
				code:          http.StatusBadRequest,
				tokenExpected: false,
				errMessage:    "W1R3: It's not you it's me.",
			},
			AuthorizeTestCase{
				testname:      "Valid User",
				name:          "admin",
				password:      "password",
				code:          http.StatusOK,
				tokenExpected: true,
				errMessage:    "W1R3: Device Admin Authorized",
			},
		}

		for _, tc := range cases {
			t.Run(tc.testname, func(t *testing.T) {
				if !adminExists(t) {
					addAdmin(t)
				}
				var admin models.User
				admin.UserName = tc.name
				admin.Password = tc.password
				response, err := api(admin, http.MethodPost, "http://localhost:8081/users/authenticate", "secretkey")
				if err != nil {
					t.Error("authenticate api call failed")
				}
				if tc.tokenExpected {
					var body goodResponse
					json.NewDecoder(response.Body).Decode(&body)
					assert.NotEmpty(t, body.Response.AuthToken, "token not returned")
					assert.Equal(t, "W1R3: Device admin Authorized", body.Message)
				} else {
					var body badResponse
					json.NewDecoder(response.Body).Decode(&body)
					assert.Equal(t, tc.errMessage, body.Message)
				}
				assert.Equal(t, tc.code, response.StatusCode)
			})
		}
	})
}

func adminExists(t *testing.T) bool {
	response, err := http.Get("http://localhost:8081/users/hasadmin")
	if err != nil {
		t.Fatal("error calling users/hasadmin", err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var body bool
	json.NewDecoder(response.Body).Decode(&body)
	return body
}

func api(data interface{}, method, url, authorization string) (*http.Response, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
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
	payload, _ := json.Marshal(admin)
	request, err := http.NewRequest(http.MethodPost, "http://localhost:8081/users/createadmin",
		bytes.NewBuffer(payload))
	assert.NotNilf(t, err, "none nil error creating http request")
	request.Header.Set("Authorization", "Bearer secretkey")
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	assert.NotNilf(t, err, "non nil err response from createadmin")
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func authenticate() (string, error) {
	var admin models.User
	admin.UserName = "admin"
	admin.Password = "password"
	response, err := api(admin, http.MethodPost, "http://localhost:8081/users/authenticate", "secretkey")
	if err != nil {
		return "", err
	}
	var body goodResponse
	json.NewDecoder(response.Body).Decode(&body)
	token := body.Response.AuthToken
	return token, nil
}
