package main

import (
	"bytes"
	"encoding/json"
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

func TestUsers(t *testing.T) {

	//	t.Run("check that admin user does not exist", func(t *testing.T) {
	//		response := checkAdminExists(t)
	//		assert.Equal(t, false, response)
	//	})
}
func TestAdminCreation(t *testing.T) {
	var admin models.UserAuthParams
	var user models.User
	admin.UserName = "admin"
	admin.Password = "password"
	t.Run("AdminCreationSuccess", func(t *testing.T) {
		if adminExists(t) {
			deleteAdmin(t)
		}
		response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&user)
		assert.Nil(t, err, err)
		assert.Equal(t, admin.UserName, user.UserName)
		assert.Equal(t, true, user.IsAdmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.True(t, adminExists(t), "Admin creation failed")
	})
	t.Run("AdminCreationFailure", func(t *testing.T) {
		if !adminExists(t) {
			addAdmin(t)
		}
		response, err := api(t, admin, http.MethodPost, "http://localhost:8081/users/createadmin", "")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var message models.ErrorResponse
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Equal(t, http.StatusUnauthorized, message.Code)
		assert.Equal(t, "W1R3: Admin already exists! ", message.Message)
	})

}

func TestGetUser(t *testing.T) {

	//ensure admin exists
	if !adminExists(t) {
		addAdmin(t)
	}
	//authenticate
	t.Run("GetUserWithValidToken", func(t *testing.T) {
		token, err := authenticate(t)
		assert.Nil(t, err, err)
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/users/admin", token)
		assert.Nil(t, err, err)
		defer response.Body.Close()
		var user models.User
		json.NewDecoder(response.Body).Decode(&user)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "admin", user.UserName)
		assert.Equal(t, true, user.IsAdmin)
	})
	t.Run("GetUserWithInvalidToken", func(t *testing.T) {
		//skip until sort out what should be returned
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/users/admin", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		///not sure what this should be
		t.Log(response.Body)
		//var something string

		//json.NewDecoder(response.Body).Decode(something)
		//assert.Equal(t, "Some error message", something)
	})
}

func TestUpdateUser(t *testing.T) {
	if !adminExists(t) {
		addAdmin(t)
	}
	token, err := authenticate(t)
	assert.Nil(t, err, err)
	var admin models.UserAuthParams
	var user models.User
	var message models.ErrorResponse
	t.Run("UpdateWrongToken", func(t *testing.T) {
		admin.UserName = "admin"
		admin.Password = "admin"
		response, err := api(t, admin, http.MethodPut, "http://localhost:8081/users/admin", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&message)
		assert.Nil(t, err, err)
		assert.Equal(t, "W1R3: Error Verifying Auth Token.", message.Message)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("UpdateSuccess", func(t *testing.T) {
		admin.UserName = "admin"
		admin.Password = "password"
		response, err := api(t, admin, http.MethodPut, "http://localhost:8081/users/admin", token)
		assert.Nil(t, err, err)
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&user)
		assert.Nil(t, err, err)
		assert.Equal(t, admin.UserName, user.UserName)
		assert.Equal(t, true, user.IsAdmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

}

func TestDeleteUser(t *testing.T) {
	if !adminExists(t) {
		addAdmin(t)
	}
	token, err := authenticate(t)
	assert.Nil(t, err, err)
	t.Run("DeleteUser-WongAdmin", func(t *testing.T) {
		//skip for now ... shouldn't panic
		t.Skip()
		function := func() {
			_, _ = api(t, "", http.MethodDelete, "http://localhost:8081/users/xxxx", token)
		}
		assert.Panics(t, function, "")
	})
	t.Run("DeleteUser-InvalidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", "secretkey")
		assert.Nil(t, err, err)
		var message models.ErrorResponse
		json.NewDecoder(response.Body).Decode(&message)
		assert.Equal(t, "W1R3: Error Verifying Auth Token.", message.Message)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("DeleteUser-ValidCredentials", func(t *testing.T) {
		response, err := api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", token)
		assert.Nil(t, err, err)
		var body string
		json.NewDecoder(response.Body).Decode(&body)
		assert.Equal(t, "admin deleted.", body)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("DeletUser-NoAdmin", func(t *testing.T) {
		//skip for now ... shouldn't panic
		t.Skip()
		function := func() {
			_, _ = api(t, "", http.MethodDelete, "http://localhost:8081/users/admin", token)
		}
		assert.Panics(t, function, "")
	})
	addAdmin(t)
}

func TestAuthenticateUser(t *testing.T) {
	cases := []AuthorizeTestCase{
		AuthorizeTestCase{
			testname:      "Invalid User",
			name:          "invaliduser",
			password:      "password",
			code:          http.StatusBadRequest,
			tokenExpected: false,
			errMessage:    "W1R3: User invaliduser not found.",
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
			code:          http.StatusUnauthorized,
			tokenExpected: false,
			errMessage:    "W1R3: Wrong Password.",
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
			defer response.Body.Close()
			if tc.tokenExpected {
				var body Success
				err = json.NewDecoder(response.Body).Decode(&body)
				assert.Nil(t, err, err)
				assert.NotEmpty(t, body.Response.AuthToken, "token not returned")
				assert.Equal(t, "W1R3: Device admin Authorized", body.Message)
			} else {
				var bad models.ErrorResponse
				json.NewDecoder(response.Body).Decode(&bad)
				assert.Equal(t, tc.errMessage, bad.Message)
			}
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
