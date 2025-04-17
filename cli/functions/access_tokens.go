package functions

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// CreateAccessToken - creates an access token for a user
func CreateAccessToken(payload *models.UserAccessToken) *models.SuccessfulUserLoginResponse {
	res := request[models.SuccessResponse](http.MethodPost, "/api/v1/users/access_token", payload)
	if res.Code != http.StatusOK {
		log.Fatalf("Error creating access token: %s", res.Message)
	}

	var token models.SuccessfulUserLoginResponse
	responseBytes, err := json.Marshal(res.Response)
	if err != nil {
		log.Fatalf("Error marshaling response: %v", err)
	}

	if err := json.Unmarshal(responseBytes, &token); err != nil {
		log.Fatalf("Error unmarshaling token: %v", err)
	}

	return &token
}

// GetAccessToken - fetch all access tokens per user
func GetAccessToken(userName string) []models.UserAccessToken {
	res := request[models.SuccessResponse](http.MethodGet, "/api/v1/users/access_token?username="+userName, nil)
	if res.Code != http.StatusOK {
		log.Fatalf("Error getting access token: %s", res.Message)
	}

	var tokens []models.UserAccessToken
	responseBytes, err := json.Marshal(res.Response)
	if err != nil {
		log.Fatalf("Error marshaling response: %v", err)
	}

	if err := json.Unmarshal(responseBytes, &tokens); err != nil {
		log.Fatalf("Error unmarshaling tokens: %v", err)
	}

	return tokens
}

// DeleteAccessToken - delete an access token
func DeleteAccessToken(id string) {
	res := request[models.SuccessResponse](http.MethodDelete, "/api/v1/users/access_token?id="+id, nil)
	if res.Code != http.StatusOK {
		log.Fatalf("Error deleting access token: %s", res.Message)
	}
}
