package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"golang.org/x/oauth2"
)

// == consts ==
const (
	node_signin_length = 64
)

var (
	auth_provider *oauth2.Config
)

// IsOauthUser - returns
func IsOauthUser(user *models.User) error {
	var currentValue, err = FetchPassValue("")
	if err != nil {
		return err
	}
	var bCryptErr = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentValue))
	return bCryptErr
}

func FetchPassValue(newValue string) (string, error) {

	type valueHolder struct {
		Value string `json:"value" bson:"value"`
	}
	newValueHolder := valueHolder{}
	var currentValue, err = logic.FetchAuthSecret()
	if err != nil {
		return "", err
	}
	var unmarshErr = json.Unmarshal([]byte(currentValue), &newValueHolder)
	if unmarshErr != nil {
		return "", unmarshErr
	}

	var b64CurrentValue, b64Err = base64.StdEncoding.DecodeString(newValueHolder.Value)
	if b64Err != nil {
		logger.Log(0, "could not decode pass")
		return "", nil
	}
	return string(b64CurrentValue), nil
}

// == private ==

func addUser(email string) error {
	var hasSuperAdmin, err = logic.HasSuperAdmin()
	if err != nil {
		slog.Error("error checking for existence of admin user during OAuth login for", "email", email, "error", err)
		return err
	} // generate random password to adapt to current model
	var newPass, fetchErr = FetchPassValue("")
	if fetchErr != nil {
		slog.Error("failed to get password", "error", fetchErr.Error())
		return fetchErr
	}
	var newUser = models.User{
		UserName: email,
		Password: newPass,
	}
	if !hasSuperAdmin { // must be first attempt, create a superadmin
		logger.Log(0, "creating superadmin")
		if err = logic.CreateSuperAdmin(&newUser); err != nil {
			slog.Error("error creating super admin from user", "email", email, "error", err)
		} else {
			slog.Info("superadmin created from user", "email", email)
		}
	} else { // otherwise add to db as admin..?
		// TODO: add ability to add users with preemptive permissions
		newUser.IsAdmin = false
		if err = logic.CreateUser(&newUser); err != nil {
			logger.Log(0, "error creating user,", email, "; user not added", "error", err.Error())
		} else {
			logger.Log(0, "user created from ", email)
		}
	}
	return nil
}

func isUserIsAllowed(username, network string, shouldAddUser bool) (*models.User, error) {

	user, err := logic.GetUser(username)
	if err != nil && shouldAddUser { // user must not exist, so try to make one
		if err = addUser(username); err != nil {
			logger.Log(0, "failed to add user", username, "during a node SSO network join on network", network)
			// response := returnErrTemplate(user.UserName, "failed to add user", state, reqKeyIf)
			// w.WriteHeader(http.StatusInternalServerError)
			// w.Write(response)
			return nil, fmt.Errorf("failed to add user to system")
		}
		logger.Log(0, "user", username, "was added during a node SSO network join on network", network)
		user, _ = logic.GetUser(username)
	}

	return user, nil
}
