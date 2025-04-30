package azure

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/idp"
	"net/http"
	"net/url"
)

type Client struct {
	clientID     string
	clientSecret string
	tenantID     string
}

func NewAzureEntraIDClient() *Client {
	settings := logic.GetServerSettings()

	return &Client{
		clientID:     settings.ClientID,
		clientSecret: settings.ClientSecret,
		tenantID:     settings.AzureTenant,
	}
}

func (a *Client) GetUsers() ([]idp.User, error) {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/users?$select=id,userPrincipalName,accountEnabled", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var users getUsersResponse
	err = json.NewDecoder(resp.Body).Decode(&users)
	if err != nil {
		return nil, err
	}

	retval := make([]idp.User, len(users.Value))
	for i, user := range users.Value {
		retval[i] = idp.User{
			ID:              user.Id,
			Username:        user.UserPrincipalName,
			AccountDisabled: user.AccountEnabled,
		}
	}

	return retval, nil
}

func (a *Client) GetGroups() ([]idp.Group, error) {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/groups?$select=id,displayName&$expand=members($select=id)", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var groups getGroupsResponse
	err = json.NewDecoder(resp.Body).Decode(&groups)
	if err != nil {
		return nil, err
	}

	retval := make([]idp.Group, len(groups.Value))
	for i, group := range groups.Value {
		retvalMembers := make([]string, len(group.Members))
		for j, member := range group.Members {
			retvalMembers[j] = member.Id
		}

		retval[i] = idp.Group{
			ID:      group.Id,
			Name:    group.DisplayName,
			Members: retvalMembers,
		}
	}

	return retval, nil
}

func (a *Client) getAccessToken() (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.tenantID)

	var data = url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", a.clientID)
	data.Set("client_secret", a.clientSecret)
	data.Set("scope", "https://graph.microsoft.com/.default")

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var tokenResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return "", err
	}

	if token, ok := tokenResp["access_token"].(string); ok {
		return token, nil
	}

	return "", errors.New("failed to get access token")
}

type getUsersResponse struct {
	OdataContext string `json:"@odata.context"`
	Value        []struct {
		Id                string `json:"id"`
		UserPrincipalName string `json:"userPrincipalName"`
		AccountEnabled    bool   `json:"accountEnabled"`
	} `json:"value"`
}

type getGroupsResponse struct {
	OdataContext string `json:"@odata.context"`
	Value        []struct {
		Id          string `json:"id"`
		DisplayName string `json:"displayName"`
		Members     []struct {
			OdataType string `json:"@odata.type"`
			Id        string `json:"id"`
		} `json:"members"`
	} `json:"value"`
}
