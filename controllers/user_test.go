package controller

import (
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func deleteAllUsers() {
	users, _ := logic.GetUsers()
	for _, user := range users {
		logic.DeleteUser(user.UserName)
	}
}

func TestHasAdmin(t *testing.T) {
	//delete all current users
	database.InitializeDatabase()
	users, _ := logic.GetUsers()
	for _, user := range users {
		success, err := logic.DeleteUser(user.UserName)
		assert.Nil(t, err)
		assert.True(t, success)
	}
	t.Run("NoUser", func(t *testing.T) {
		found, err := logic.HasAdmin()
		assert.Nil(t, err)
		assert.False(t, found)
	})
	t.Run("No admin user", func(t *testing.T) {
		var user = models.User{"noadmin", "password", nil, false, nil}
		err := logic.CreateUser(&user)
		assert.Nil(t, err)
		found, err := logic.HasAdmin()
		assert.Nil(t, err)
		assert.False(t, found)
	})
	t.Run("admin user", func(t *testing.T) {
		var user = models.User{"admin", "password", nil, true, nil}
		err := logic.CreateUser(&user)
		assert.Nil(t, err)
		found, err := logic.HasAdmin()
		assert.Nil(t, err)
		assert.True(t, found)
	})
	t.Run("multiple admins", func(t *testing.T) {
		var user = models.User{"admin1", "password", nil, true, nil}
		 err := logic.CreateUser(&user)
		assert.Nil(t, err)
		found, err := logic.HasAdmin()
		assert.Nil(t, err)
		assert.True(t, found)
	})
}

func TestCreateUser(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	user := models.User{"admin", "password", nil, true, nil}
	t.Run("NoUser", func(t *testing.T) {
		err := logic.CreateUser(&user)
		assert.Nil(t, err)
	})
	t.Run("UserExists", func(t *testing.T) {
		err := logic.CreateUser(&user)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "user exists")
	})
}

func TestCreateAdmin(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	var user models.User
	t.Run("NoAdmin", func(t *testing.T) {
		user.UserName = "admin"
		user.Password = "password"
		err := logic.CreateAdmin(&user)
		assert.Nil(t, err)
	})
	t.Run("AdminExists", func(t *testing.T) {
		user.UserName = "admin2"
		user.Password = "password1"
		err := logic.CreateAdmin(&user)
		assert.EqualError(t, err, "admin user already exists")
	})
}

func TestDeleteUser(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	t.Run("NonExistent User", func(t *testing.T) {
		deleted, err := logic.DeleteUser("admin")
		assert.EqualError(t, err, "user does not exist")
		assert.False(t, deleted)
	})
	t.Run("Existing User", func(t *testing.T) {
		user := models.User{"admin", "password", nil, true, nil}
		logic.CreateUser(&user)
		deleted, err := logic.DeleteUser("admin")
		assert.Nil(t, err)
		assert.True(t, deleted)
	})
}

func TestValidateUser(t *testing.T) {
	database.InitializeDatabase()
	var user models.User
	t.Run("Valid Create", func(t *testing.T) {
		user.UserName = "admin"
		user.Password = "validpass"
		err := logic.ValidateUser(&user)
		assert.Nil(t, err)
	})
	t.Run("Valid Update", func(t *testing.T) {
		user.UserName = "admin"
		user.Password = "password"
		err := logic.ValidateUser(&user)
		assert.Nil(t, err)
	})
	t.Run("Invalid UserName", func(t *testing.T) {
		t.Skip()
		user.UserName = "*invalid"
		err := logic.ValidateUser(&user)
		assert.Error(t, err)
		//assert.Contains(t, err.Error(), "Field validation for 'UserName' failed")
	})
	t.Run("Short UserName", func(t *testing.T) {
		t.Skip()
		user.UserName = "1"
		err := logic.ValidateUser(&user)
		assert.NotNil(t, err)
		//assert.Contains(t, err.Error(), "Field validation for 'UserName' failed")
	})
	t.Run("Empty UserName", func(t *testing.T) {
		t.Skip()
		user.UserName = ""
		err := logic.ValidateUser(&user)
		assert.EqualError(t, err, "some string")
		//assert.Contains(t, err.Error(), "Field validation for 'UserName' failed")
	})
	t.Run("EmptyPassword", func(t *testing.T) {
		user.Password = ""
		err := logic.ValidateUser(&user)
		assert.EqualError(t, err, "Key: 'User.Password' Error:Field validation for 'Password' failed on the 'required' tag")
	})
	t.Run("ShortPassword", func(t *testing.T) {
		user.Password = "123"
		err := logic.ValidateUser(&user)
		assert.EqualError(t, err, "Key: 'User.Password' Error:Field validation for 'Password' failed on the 'min' tag")
	})
}

func TestGetUser(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	t.Run("NonExistantUser", func(t *testing.T) {
		admin, err := logic.GetUser("admin")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, "", admin.UserName)
	})
	t.Run("UserExisits", func(t *testing.T) {
		user := models.User{"admin", "password", nil, true, nil}
		logic.CreateUser(&user)
		admin, err := logic.GetUser("admin")
		assert.Nil(t, err)
		assert.Equal(t, user.UserName, admin.UserName)
	})
}

func TestGetUsers(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	t.Run("NonExistantUser", func(t *testing.T) {
		admin, err := logic.GetUsers()
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, []models.ReturnUser(nil), admin)
	})
	t.Run("UserExisits", func(t *testing.T) {
		user := models.User{"admin", "password", nil, true, nil}
		logic.CreateUser(&user)
		admins, err := logic.GetUsers()
		assert.Nil(t, err)
		assert.Equal(t, user.UserName, admins[0].UserName)
	})
	t.Run("MulipleUsers", func(t *testing.T) {
		user := models.User{"user", "password", nil, true, nil}
		logic.CreateUser(&user)
		admins, err := logic.GetUsers()
		assert.Nil(t, err)
		for _, u := range admins {
			if u.UserName == "admin" {
				assert.Equal(t, "admin", u.UserName)
			} else {
				assert.Equal(t, user.UserName, u.UserName)
			}
		}
	})

}

func TestUpdateUser(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	user := models.User{"admin", "password", nil, true, nil}
	newuser := models.User{"hello", "world", []string{"wirecat, netmaker"}, true, []string{}}
	t.Run("NonExistantUser", func(t *testing.T) {
		admin, err := logic.UpdateUser(&newuser, &user)
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, "", admin.UserName)
	})

	t.Run("UserExists", func(t *testing.T) {
		logic.CreateUser(&user)
		admin, err := logic.UpdateUser(&newuser, &user)
		assert.Nil(t, err)
		assert.Equal(t, newuser.UserName, admin.UserName)
	})
}

// func TestValidateUserToken(t *testing.T) {
// 	t.Run("EmptyToken", func(t *testing.T) {
// 		err := ValidateUserToken("", "", false)
// 		assert.NotNil(t, err)
// 		assert.Equal(t, "Missing Auth Token.", err.Error())
// 	})
// 	t.Run("InvalidToken", func(t *testing.T) {
// 		err := ValidateUserToken("Bearer: badtoken", "", false)
// 		assert.NotNil(t, err)
// 		assert.Equal(t, "Error Verifying Auth Token", err.Error())
// 	})
// 	t.Run("InvalidUser", func(t *testing.T) {
// 		t.Skip()
// 		err := ValidateUserToken("Bearer: secretkey", "baduser", false)
// 		assert.NotNil(t, err)
// 		assert.Equal(t, "Error Verifying Auth Token", err.Error())
// 		//need authorization
// 	})
// 	t.Run("ValidToken", func(t *testing.T) {
// 		err := ValidateUserToken("Bearer: secretkey", "", true)
// 		assert.Nil(t, err)
// 	})
// }

func TestVerifyAuthRequest(t *testing.T) {
	database.InitializeDatabase()
	deleteAllUsers()
	var authRequest models.UserAuthParams
	t.Run("EmptyUserName", func(t *testing.T) {
		authRequest.UserName = ""
		authRequest.Password = "Password"
		jwt, err := logic.VerifyAuthRequest(authRequest)
		assert.Equal(t, "", jwt)
		assert.EqualError(t, err, "username can't be empty")
	})
	t.Run("EmptyPassword", func(t *testing.T) {
		authRequest.UserName = "admin"
		authRequest.Password = ""
		jwt, err := logic.VerifyAuthRequest(authRequest)
		assert.Equal(t, "", jwt)
		assert.EqualError(t, err, "password can't be empty")
	})
	t.Run("NonExistantUser", func(t *testing.T) {
		authRequest.UserName = "admin"
		authRequest.Password = "password"
		jwt, err := logic.VerifyAuthRequest(authRequest)
		assert.Equal(t, "", jwt)
		assert.EqualError(t, err, "error retrieving user from db: could not find any records")
	})
	t.Run("Non-Admin", func(t *testing.T) {
		user := models.User{"nonadmin", "somepass", nil, false, []string{}}
		logic.CreateUser(&user)
		authRequest := models.UserAuthParams{"nonadmin", "somepass"}
		jwt, err := logic.VerifyAuthRequest(authRequest)
		assert.NotNil(t, jwt)
		assert.Nil(t, err)
	})
	t.Run("WrongPassword", func(t *testing.T) {
		user := models.User{"admin", "password", nil, false, []string{}}
		logic.CreateUser(&user)
		authRequest := models.UserAuthParams{"admin", "badpass"}
		jwt, err := logic.VerifyAuthRequest(authRequest)
		assert.Equal(t, "", jwt)
		assert.EqualError(t, err, "incorrect credentials")
	})
	t.Run("Success", func(t *testing.T) {
		authRequest := models.UserAuthParams{"admin", "password"}
		jwt, err := logic.VerifyAuthRequest(authRequest)
		assert.Nil(t, err)
		assert.NotNil(t, jwt)
	})
}
