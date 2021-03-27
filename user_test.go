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

	"github.com/chromedp/cdproto/database"
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

type databaseError struct {
	Inner  *int
	Errors int
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
}
func TestAdminCreation(t *testing.T) {
	t.Skip()
	var admin, user models.User
	admin.UserName = "admin"
	admin.Password = "password"
	if !adminExists(t) {
		t.Run("AdminCreationValid", func(t *testing.T) {
			response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
			assert.NotNil(t, err, "create admin")
			if err != nil {
				t.Fatal("....")
			}
			defer response.Body.Close()
			json.NewDecoder(response.Body).Decode(&user)
			assert.Equal(t, admin.UserName, user.UserName)
			assert.Equal(t, true, user.IsAdmin)
			assert.Equal(t, http.StatusOK, response.StatusCode)
			assert.True(t, adminExists(t), "Admin creation failed")
		})
		t.Run("AdminCreationInvalid", func(t *testing.T) {
			response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
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
			response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
			assert.NotNil(t, err, "createadmin")
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
			response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
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
}

func TestGetUser(t *testing.T) {

	t.Skip()
	//ensure admin exists
	if !adminExists(t) {
		t.Error("admin account does not exist")
		return
	}
	//authenticate
	t.Run("GetUser ValidToken", func(t *testing.T) {
		token, err := authenticate(t)
		if err != nil {
			t.Error("could not authenticate")
		}
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/users/admin", token)
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
	t.Run("GetUser InvalidToken", func(t *testing.T) {
		//skip until sort out what should be returned
		t.Skip()
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/users/admin", "xxxx")
		if err != nil {
			t.Error("error getting user")
		}
		defer response.Body.Close()
		///not sure what this should be
		var something string
		json.NewDecoder(response.Body).Decode(something)
		assert.Equal(t, "Some error message", something)
	})
}

func TestUpdateUser(t *testing.T) {
	t.Skip()
	if !adminExists(t) {
		addAdmin(t)
	}
	//token, err := authenticate(t)
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
}

func TestDeleteUser(t *testing.T) {
	expectedError := databaseError{
		Inner:  nil,
		Errors: 1,
	}

	if !adminExists(t) {
		addAdmin(t)
	}
	token, err := authenticate(t)
	if err != nil {
		t.Error(err)
	}
	t.Run("DeleteUser-WongAdmin", func(t *testing.T) {
		t.Skip()
		function := func() {
			_, _ = api(t, "", http.MethodDelete, "http://localhost:8081/users/xxxx", token)
		}
		assert.Panics(t, function, "")
	})
	t.Run("DeleteUser-InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", "secretkey")
		if err != nil {
			t.Error(err)
		}
		var body database.Error
		json.NewDecoder(response.Body).Decode(body)
		assert.Equal(t, expectedError, body)
	})
	t.Run("DeleteUser-ValidCredentials", func(t *testing.T) {
		if err != nil {
			t.Error(err)
		}
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", token)
		if err != nil {
			t.Error(err)
		}
		var body string
		json.NewDecoder(response.Body).Decode(body)
		assert.Equal(t, "admin deleted.", body)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("DeletUser-NoAdmin", func(t *testing.T) {
		t.Skip()
		function := func() {
			_, _ = api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", token)
		}
		assert.Panics(t, function, "")
	})
}

func TestAuthenticateUser(t *testing.T) {
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

	if !adminExists(t) {
		addAdmin(t)
	}
	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			var admin models.User
			admin.UserName = tc.name
			admin.Password = tc.password
			response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/authenticate", "secretkey")
			assert.Nil(t, err, err)
			if tc.tokenExpected {
				var body goodResponse
				json.NewDecoder(response.Body).Decode(&body)
				assert.NotEmpty(t, body.Response.AuthToken, "token not returned")
				assert.Equal(t, "W1R3: Device admin Authorized", body.Message)
			} //else {
			//	var bad badResponse
			//	json.NewDecoder(response.Body).Decode(&bad)
			//	assert.Equal(t, tc.errMessage, bad.Message)
			//}
			assert.Equal(t, tc.code, response.StatusCode)
		})
	}
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
	payload, err := json.Marshal(data)
	assert.Nil(t, err, err)
	return nil, err
	request, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	assert.Nil(t, err, err)
	return nil, err
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
	response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "secretkey")
	assert.Nil(t, err, err)
	t.Log(response)
	//assert.Equal(t, http.StatusOK, response.StatusCode)
}

func authenticate(t *testing.T) (string, error) {
	var admin models.User
	admin.UserName = "admin"
	admin.Password = "password"
	response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/authenticate", "secretkey")
	assert.NotNil(t, err, "authenticate")
	var body goodResponse
	json.NewDecoder(response.Body).Decode(&body)
	token := body.Response.AuthToken
	return token, nil
}
