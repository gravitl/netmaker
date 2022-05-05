package config

import (
	"encoding/base64"
	"encoding/json"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

var (
	// GuiActive - indicates if gui is active or not
	GuiActive = false
	// GuiRun - holds function for main to call
	GuiRun interface{}
)

// ParseAccessToken - used to parse the base64 encoded access token
func ParseAccessToken(token string) (*models.AccessToken, error) {
	tokenbytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		logger.Log(0, "error decoding token", err.Error())
		return nil, err
	}
	var accesstoken models.AccessToken
	if err := json.Unmarshal(tokenbytes, &accesstoken); err != nil {
		logger.Log(0, "error decoding token", err.Error())
		return nil, err
	}
	return &accesstoken, nil
}
