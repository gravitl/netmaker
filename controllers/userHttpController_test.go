package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	mongoconn.ConnectDatabase()
	var gconf models.GlobalConfig
	gconf.ServerGRPC = "localhost:8081"
	gconf.PortGRPC = "50051"
	//err := SetGlobalConfig(gconf)
	collection := mongoconn.Client.Database("netmaker").Collection("config")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	//create, _, err := functions.GetGlobalConfig()
	_, err := collection.InsertOne(ctx, gconf)
	if err != nil {
		panic("could not create config store")
	}
	os.Exit(m.Run())
}

func TestHasAdmin(t *testing.T) {
	_, err := DeleteUser("admin")
	assert.Nil(t, err)
	user := models.User{"admin", "password", true}
	_, err = CreateUser(user)
	assert.Nil(t, err, err)
	t.Run("AdminExists", func(t *testing.T) {
		found, err := HasAdmin()
		assert.Nil(t, err, err)
		assert.True(t, found)
	})
	t.Run("NoUser", func(t *testing.T) {
		_, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		found, err := HasAdmin()
		assert.Nil(t, err, err)
		assert.False(t, found)
	})
}

func TestCreateUser(t *testing.T) {
	user := models.User{"admin", "password", true}
	t.Run("NoUser", func(t *testing.T) {
		_, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		admin, err := CreateUser(user)
		assert.Nil(t, err, err)
		assert.Equal(t, user.UserName, admin.UserName)
	})
	t.Run("AdminExists", func(t *testing.T) {
		_, err := CreateUser(user)
		assert.NotNil(t, err, err)
		assert.Equal(t, "Admin already Exists", err.Error())
	})
}

func TestDeleteUser(t *testing.T) {
	hasadmin, err := HasAdmin()
	assert.Nil(t, err, err)
	if !hasadmin {
		user := models.User{"admin", "pasword", true}
		_, err := CreateUser(user)
		assert.Nil(t, err, err)
	}
	t.Run("ExistingUser", func(t *testing.T) {
		deleted, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		assert.True(t, deleted)
		t.Log(deleted, err)
	})
	t.Run("NonExistantUser", func(t *testing.T) {
		deleted, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		assert.False(t, deleted)
	})
}

func TestValidateUser(t *testing.T) {
	var user models.User
	t.Run("ValidCreate", func(t *testing.T) {
		user.UserName = "admin"
		user.Password = "validpass"
		err := ValidateUser("create", user)
		assert.Nil(t, err, err)
	})
	t.Run("ValidUpdate", func(t *testing.T) {
		user.UserName = "admin"
		user.Password = "password"
		err := ValidateUser("update", user)
		assert.Nil(t, err, err)
	})
	t.Run("InvalidUserName", func(t *testing.T) {
		user.UserName = "invalid*"
		err := ValidateUser("update", user)
		assert.NotNil(t, err, err)
	})
	t.Run("ShortUserName", func(t *testing.T) {
		user.UserName = "12"
		err := ValidateUser("create", user)
		assert.NotNil(t, err, err)
	})
	t.Run("EmptyPassword", func(t *testing.T) {
		user.Password = ""
		err := ValidateUser("create", user)
		assert.NotNil(t, err, err)
	})
	t.Run("ShortPassword", func(t *testing.T) {
		user.Password = "123"
		err := ValidateUser("create", user)
		assert.NotNil(t, err, err)
	})
}

func TestGetUser(t *testing.T) {
	t.Run("UserExisits", func(t *testing.T) {
		user := models.User{"admin", "password", true}
		hasadmin, err := HasAdmin()
		assert.Nil(t, err, err)
		if !hasadmin {
			_, err := CreateUser(user)
			assert.Nil(t, err, err)
		}
		admin, err := GetUser("admin")
		assert.Nil(t, err, err)
		assert.Equal(t, user.UserName, admin.UserName)
	})
	t.Run("NonExistantUser", func(t *testing.T) {
		_, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		admin, err := GetUser("admin")
		assert.Equal(t, "mongo: no documents in result", err.Error())
		assert.Equal(t, "", admin.UserName)
	})
}

func TestUpdateUser(t *testing.T) {
	user := models.User{"admin", "password", true}
	newuser := models.User{"hello", "world", true}
	t.Run("UserExisits", func(t *testing.T) {
		_, err := DeleteUser("admin")
		_, err = CreateUser(user)
		assert.Nil(t, err, err)
		admin, err := UpdateUser(newuser, user)
		assert.Nil(t, err, err)
		assert.Equal(t, newuser.UserName, admin.UserName)
	})
	t.Run("NonExistantUser", func(t *testing.T) {
		_, err := DeleteUser("hello")
		assert.Nil(t, err, err)
		_, err = UpdateUser(newuser, user)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
}

func TestValidateToken(t *testing.T) {
	t.Run("EmptyToken", func(t *testing.T) {
		err := ValidateToken("")
		assert.NotNil(t, err, err)
		assert.Equal(t, "Missing Auth Token.", err.Error())
	})
	t.Run("InvalidToken", func(t *testing.T) {
		err := ValidateToken("Bearer: badtoken")
		assert.NotNil(t, err, err)
		assert.Equal(t, "Error Verifying Auth Token", err.Error())
	})
	t.Run("InvalidUser", func(t *testing.T) {
		t.Skip()
		//need authorization
	})
	t.Run("ValidToken", func(t *testing.T) {
		err := ValidateToken("Bearer: secretkey")
		assert.Nil(t, err, err)
	})
}

func TestVerifyAuthRequest(t *testing.T) {
	var authRequest models.UserAuthParams
	t.Run("EmptyUserName", func(t *testing.T) {
		authRequest.UserName = ""
		authRequest.Password = "Password"
		jwt, err := VerifyAuthRequest(authRequest)
		assert.NotNil(t, err, err)
		assert.Equal(t, "", jwt)
		assert.Equal(t, "Username can't be empty", err.Error())
	})
	t.Run("EmptyPassword", func(t *testing.T) {
		authRequest.UserName = "admin"
		authRequest.Password = ""
		jwt, err := VerifyAuthRequest(authRequest)
		assert.NotNil(t, err, err)
		assert.Equal(t, "", jwt)
		assert.Equal(t, "Password can't be empty", err.Error())
	})
	t.Run("NonExistantUser", func(t *testing.T) {
		_, err := DeleteUser("admin")
		authRequest.UserName = "admin"
		authRequest.Password = "password"
		jwt, err := VerifyAuthRequest(authRequest)
		assert.NotNil(t, err, err)
		assert.Equal(t, "", jwt)
		assert.Equal(t, "User admin not found", err.Error())
	})
	t.Run("Non-Admin", func(t *testing.T) {
		//can't create a user that is not a an admin
		t.Skip()
		user := models.User{"admin", "admin", false}
		_, err := CreateUser(user)
		assert.Nil(t, err)
		authRequest := models.UserAuthParams{"admin", "admin"}
		jwt, err := VerifyAuthRequest(authRequest)
		assert.NotNil(t, err, err)
		assert.Equal(t, "", jwt)
		assert.Equal(t, "User is not an admin", err.Error())
	})
	t.Run("WrongPassword", func(t *testing.T) {
		_, err := DeleteUser("admin")
		user := models.User{"admin", "password", true}
		_, err = CreateUser(user)
		assert.Nil(t, err)
		authRequest := models.UserAuthParams{"admin", "badpass"}
		jwt, err := VerifyAuthRequest(authRequest)
		assert.NotNil(t, err, err)
		assert.Equal(t, "", jwt)
		assert.Equal(t, "Wrong Password", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		authRequest := models.UserAuthParams{"admin", "password"}
		jwt, err := VerifyAuthRequest(authRequest)
		assert.Nil(t, err, err)
		assert.NotNil(t, jwt)
	})
}
