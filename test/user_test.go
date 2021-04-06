package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

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
		response, err := api(t, "", http.MethodGet, "http://localhost:8081/users/admin", "secretkey")
		assert.Nil(t, err, err)
		defer response.Body.Close()
		t.Log(response.Body)
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
	t.Run("DeleteUser-NoAdmin", func(t *testing.T) {
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
			testname:      "Invalid Password",
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
