package controller

// TODO: Need Update Tests for New User Mgmt
// func deleteAllUsers(t *testing.T) {
// 	t.Helper()
// 	users, _ := logic.GetUsers()
// 	for _, user := range users {
// 		if _, err := logic.DeleteUser(user.UserName); err != nil {
// 			t.Fatal(err)
// 		}
// 	}
// }

// func TestGetUserNoHashedPassword(t *testing.T) {
// 	// prepare existing user base
// 	user := models.User{UserName: "freddie", Password: "password"}
// 	haveOnlyOneUser(t, user)

// 	// prepare request
// 	rec, req := prepareUserRequest(t, models.User{}, user.UserName)

// 	// test response
// 	getUser(rec, req)
// 	assertUserNameButNoPassword(t, rec.Body, user.UserName)
// }

// func TestCreateAdminNoHashedPassword(t *testing.T) {
// 	// prepare existing user base
// 	deleteAllUsers(t)

// 	// prepare request
// 	user := models.User{UserName: "jonathan", Password: "password"}
// 	rec, req := prepareUserRequest(t, user, "")

// 	// test response
// 	createSuperAdmin(rec, req)
// 	assertUserNameButNoPassword(t, rec.Body, user.UserName)
// }

// func prepareUserRequest(t *testing.T, userForBody models.User, userNameForParam string) (*httptest.ResponseRecorder, *http.Request) {
// 	bits, err := json.Marshal(userForBody)
// 	assert.Nil(t, err)
// 	body := bytes.NewReader(bits)
// 	rec := httptest.NewRecorder()
// 	req := httptest.NewRequest("ANY", "https://example.com", body) // only the body matters here
// 	req = mux.SetURLVars(req, map[string]string{"username": userNameForParam})
// 	req.Header.Set("user", userForBody.UserName)
// 	return rec, req
// }

// func haveOnlyOneUser(t *testing.T, user models.User) {
// 	deleteAllUsers(t)
// 	var err error
// 	if user.PlatformRoleID == models.SuperAdminRole {
// 		err = logic.CreateSuperAdmin(&user)
// 	} else {
// 		err = logic.CreateUser(&user)
// 	}
// 	assert.Nil(t, err)
// }

// func assertUserNameButNoPassword(t *testing.T, r io.Reader, userName string) {
// 	var resp models.User
// 	err := json.NewDecoder(r).Decode(&resp)
// 	assert.Nil(t, err)
// 	assert.Equal(t, userName, resp.UserName)
// 	assert.Empty(t, resp.Password)
// }

// func TestHasSuperAdmin(t *testing.T) {
// 	// delete all current users
// 	users, _ := logic.GetUsers()
// 	for _, user := range users {
// 		success, err := logic.DeleteUser(user.UserName)
// 		assert.Nil(t, err)
// 		assert.True(t, success)
// 	}
// 	t.Run("NoUser", func(t *testing.T) {
// 		found, err := logic.HasSuperAdmin()
// 		assert.Nil(t, err)
// 		assert.False(t, found)
// 	})
// 	t.Run("No superadmin user", func(t *testing.T) {
// 		var user = models.User{UserName: "nosuperadmin", Password: "password"}
// 		err := logic.CreateUser(&user)
// 		assert.Nil(t, err)
// 		found, err := logic.HasSuperAdmin()
// 		assert.Nil(t, err)
// 		assert.False(t, found)
// 	})
// 	t.Run("superadmin user", func(t *testing.T) {
// 		var user = models.User{UserName: "superadmin", Password: "password", PlatformRoleID: models.SuperAdminRole}
// 		err := logic.CreateUser(&user)
// 		assert.Nil(t, err)
// 		found, err := logic.HasSuperAdmin()
// 		assert.Nil(t, err)
// 		assert.True(t, found)
// 	})
// 	t.Run("multiple superadmins", func(t *testing.T) {
// 		var user = models.User{UserName: "superadmin1", Password: "password", PlatformRoleID: models.SuperAdminRole}
// 		err := logic.CreateUser(&user)
// 		assert.Nil(t, err)
// 		found, err := logic.HasSuperAdmin()
// 		assert.Nil(t, err)
// 		assert.True(t, found)
// 	})
// }

// func TestCreateUser(t *testing.T) {
// 	deleteAllUsers(t)
// 	user := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}
// 	t.Run("NoUser", func(t *testing.T) {
// 		err := logic.CreateUser(&user)
// 		assert.Nil(t, err)
// 	})
// 	t.Run("UserExists", func(t *testing.T) {
// 		err := logic.CreateUser(&user)
// 		assert.NotNil(t, err)
// 		assert.EqualError(t, err, "user exists")
// 	})
// }

// func TestCreateSuperAdmin(t *testing.T) {
// 	deleteAllUsers(t)
// 	logic.ClearSuperUserCache()
// 	var user models.User
// 	t.Run("NoSuperAdmin", func(t *testing.T) {
// 		user.UserName = "admin"
// 		user.Password = "password"
// 		err := logic.CreateSuperAdmin(&user)
// 		assert.Nil(t, err)
// 	})
// 	t.Run("SuperAdminExists", func(t *testing.T) {
// 		user.UserName = "admin2"
// 		user.Password = "password1"
// 		err := logic.CreateSuperAdmin(&user)
// 		assert.EqualError(t, err, "superadmin user already exists")
// 	})
// }

// func TestDeleteUser(t *testing.T) {
// 	deleteAllUsers(t)
// 	t.Run("NonExistent User", func(t *testing.T) {
// 		deleted, err := logic.DeleteUser("admin")
// 		assert.EqualError(t, err, "user does not exist")
// 		assert.False(t, deleted)
// 	})
// 	t.Run("Existing User", func(t *testing.T) {
// 		user := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}
// 		if err := logic.CreateUser(&user); err != nil {
// 			t.Fatal(err)
// 		}
// 		deleted, err := logic.DeleteUser("admin")
// 		assert.Nil(t, err)
// 		assert.True(t, deleted)
// 	})
// }

// func TestValidateUser(t *testing.T) {
// 	var user models.User
// 	t.Run("Valid Create", func(t *testing.T) {
// 		user.UserName = "admin"
// 		user.Password = "validpass"
// 		err := logic.ValidateUser(&user)
// 		assert.Nil(t, err)
// 	})
// 	t.Run("Valid Update", func(t *testing.T) {
// 		user.UserName = "admin"
// 		user.Password = "password"
// 		err := logic.ValidateUser(&user)
// 		assert.Nil(t, err)
// 	})
// 	t.Run("Invalid UserName", func(t *testing.T) {
// 		t.Skip()
// 		user.UserName = "*invalid"
// 		err := logic.ValidateUser(&user)
// 		assert.Error(t, err)
// 		// assert.Contains(t, err.Error(), "Field validation for 'UserName' failed")
// 	})
// 	t.Run("Short UserName", func(t *testing.T) {
// 		t.Skip()
// 		user.UserName = "1"
// 		err := logic.ValidateUser(&user)
// 		assert.NotNil(t, err)
// 		// assert.Contains(t, err.Error(), "Field validation for 'UserName' failed")
// 	})
// 	t.Run("Empty UserName", func(t *testing.T) {
// 		t.Skip()
// 		user.UserName = ""
// 		err := logic.ValidateUser(&user)
// 		assert.EqualError(t, err, "some string")
// 		// assert.Contains(t, err.Error(), "Field validation for 'UserName' failed")
// 	})
// 	t.Run("EmptyPassword", func(t *testing.T) {
// 		user.Password = ""
// 		err := logic.ValidateUser(&user)
// 		assert.EqualError(t, err, "Key: 'User.Password' Error:Field validation for 'Password' failed on the 'required' tag")
// 	})
// 	t.Run("ShortPassword", func(t *testing.T) {
// 		user.Password = "123"
// 		err := logic.ValidateUser(&user)
// 		assert.EqualError(t, err, "Key: 'User.Password' Error:Field validation for 'Password' failed on the 'min' tag")
// 	})
// }

// func TestGetUser(t *testing.T) {
// 	deleteAllUsers(t)

// 	user := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}

// 	t.Run("NonExistantUser", func(t *testing.T) {
// 		admin, err := logic.GetUser("admin")
// 		assert.EqualError(t, err, "could not find any records")
// 		assert.Equal(t, "", admin.UserName)
// 	})
// 	t.Run("UserExisits", func(t *testing.T) {
// 		if err := logic.CreateUser(&user); err != nil {
// 			t.Error(err)
// 		}
// 		admin, err := logic.GetUser("admin")
// 		assert.Nil(t, err)
// 		assert.Equal(t, user.UserName, admin.UserName)
// 	})
// }

// func TestGetUsers(t *testing.T) {
// 	deleteAllUsers(t)

// 	adminUser := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}
// 	user := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}

// 	t.Run("NonExistantUser", func(t *testing.T) {
// 		admin, err := logic.GetUsers()
// 		assert.EqualError(t, err, "could not find any records")
// 		assert.Equal(t, []models.ReturnUser(nil), admin)
// 	})
// 	t.Run("UserExisits", func(t *testing.T) {
// 		user.UserName = "anotheruser"
// 		if err := logic.CreateUser(&adminUser); err != nil {
// 			t.Error(err)
// 		}
// 		admins, err := logic.GetUsers()
// 		assert.Nil(t, err)
// 		assert.Equal(t, adminUser.UserName, admins[0].UserName)
// 	})
// 	t.Run("MulipleUsers", func(t *testing.T) {
// 		if err := logic.CreateUser(&user); err != nil {
// 			t.Error(err)
// 		}
// 		admins, err := logic.GetUsers()
// 		assert.Nil(t, err)
// 		for _, u := range admins {
// 			if u.UserName == "admin" {
// 				assert.Equal(t, true, u.IsAdmin)
// 			} else {
// 				assert.Equal(t, user.UserName, u.UserName)
// 				assert.Equal(t, user.PlatformRoleID, u.PlatformRoleID)
// 			}
// 		}
// 	})

// }

// func TestUpdateUser(t *testing.T) {
// 	deleteAllUsers(t)
// 	user := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}
// 	newuser := models.User{UserName: "hello", Password: "world", PlatformRoleID: models.AdminRole}
// 	t.Run("NonExistantUser", func(t *testing.T) {
// 		admin, err := logic.UpdateUser(&newuser, &user)
// 		assert.EqualError(t, err, "could not find any records")
// 		assert.Equal(t, "", admin.UserName)
// 	})

// 	t.Run("UserExists", func(t *testing.T) {
// 		if err := logic.CreateUser(&user); err != nil {
// 			t.Error(err)
// 		}
// 		admin, err := logic.UpdateUser(&newuser, &user)
// 		assert.Nil(t, err)
// 		assert.Equal(t, newuser.UserName, admin.UserName)
// 	})
// }

// // func TestValidateUserToken(t *testing.T) {
// // 	t.Run("EmptyToken", func(t *testing.T) {
// // 		err := ValidateUserToken("", "", false)
// // 		assert.NotNil(t, err)
// // 		assert.Equal(t, "Missing Auth Token.", err.Error())
// // 	})
// // 	t.Run("InvalidToken", func(t *testing.T) {
// // 		err := ValidateUserToken("Bearer: badtoken", "", false)
// // 		assert.NotNil(t, err)
// // 		assert.Equal(t, "Error Verifying Auth Token", err.Error())
// // 	})
// // 	t.Run("InvalidUser", func(t *testing.T) {
// // 		t.Skip()
// // 		err := ValidateUserToken("Bearer: secretkey", "baduser", false)
// // 		assert.NotNil(t, err)
// // 		assert.Equal(t, "Error Verifying Auth Token", err.Error())
// // 		//need authorization
// // 	})
// // 	t.Run("ValidToken", func(t *testing.T) {
// // 		err := ValidateUserToken("Bearer: secretkey", "", true)
// // 		assert.Nil(t, err)
// // 	})
// // }

// func TestVerifyAuthRequest(t *testing.T) {
// 	deleteAllUsers(t)
// 	user := models.User{UserName: "admin", Password: "password", PlatformRoleID: models.AdminRole}
// 	var authRequest models.UserAuthParams
// 	t.Run("EmptyUserName", func(t *testing.T) {
// 		authRequest.UserName = ""
// 		authRequest.Password = "Password"
// 		jwt, err := logic.VerifyAuthRequest(authRequest)
// 		assert.Equal(t, "", jwt)
// 		assert.EqualError(t, err, "username can't be empty")
// 	})
// 	t.Run("EmptyPassword", func(t *testing.T) {
// 		authRequest.UserName = "admin"
// 		authRequest.Password = ""
// 		jwt, err := logic.VerifyAuthRequest(authRequest)
// 		assert.Equal(t, "", jwt)
// 		assert.EqualError(t, err, "password can't be empty")
// 	})
// 	t.Run("NonExistantUser", func(t *testing.T) {
// 		authRequest.UserName = "admin"
// 		authRequest.Password = "password"
// 		jwt, err := logic.VerifyAuthRequest(authRequest)
// 		assert.Equal(t, "", jwt)
// 		assert.EqualError(t, err, "incorrect credentials")
// 	})
// 	t.Run("Non-Admin", func(t *testing.T) {
// 		user.PlatformRoleID = models.ServiceUser
// 		user.Password = "somepass"
// 		user.UserName = "nonadmin"
// 		if err := logic.CreateUser(&user); err != nil {
// 			t.Error(err)
// 		}
// 		authRequest := models.UserAuthParams{UserName: "nonadmin", Password: "somepass"}
// 		jwt, err := logic.VerifyAuthRequest(authRequest)
// 		assert.NotNil(t, jwt)
// 		assert.Nil(t, err)
// 	})
// 	t.Run("WrongPassword", func(t *testing.T) {
// 		user := models.User{UserName: "admin", Password: "password"}
// 		if err := logic.CreateUser(&user); err != nil {
// 			t.Error(err)
// 		}
// 		authRequest := models.UserAuthParams{UserName: "admin", Password: "badpass"}
// 		jwt, err := logic.VerifyAuthRequest(authRequest)
// 		assert.Equal(t, "", jwt)
// 		assert.EqualError(t, err, "incorrect credentials")
// 	})
// 	t.Run("Success", func(t *testing.T) {
// 		authRequest := models.UserAuthParams{UserName: "admin", Password: "password"}
// 		jwt, err := logic.VerifyAuthRequest(authRequest)
// 		assert.Nil(t, err)
// 		assert.NotNil(t, jwt)
// 	})
// }
