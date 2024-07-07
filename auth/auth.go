package auth

import (
	"encoding/base64"
	"encoding/json"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/crypto/bcrypt"
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

func isUserIsAllowed(username, network string) (*models.User, error) {

	user, err := logic.GetUser(username)
	if err != nil { // user must not exist, so try to make one
		return &models.User{}, err
	}

	return user, nil
}
