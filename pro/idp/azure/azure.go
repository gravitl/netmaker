package azure

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/idp"
)

type Client struct {
	clientID     string
	clientSecret string
	tenantID     string
}

func NewAzureEntraIDClient(clientID, clientSecret, tenantID string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
	}
}

func NewAzureEntraIDClientFromSettings() *Client {
	settings := logic.GetServerSettings()

	return NewAzureEntraIDClient(settings.ClientID, settings.ClientSecret, settings.AzureTenant)
}

func (a *Client) Verify() error {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/users?$select=id,userPrincipalName,displayName,accountEnabled&$top=1", nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var users getUsersResponse
	err = json.NewDecoder(resp.Body).Decode(&users)
	if err != nil {
		return err
	}

	if users.Error.Code != "" {
		return errors.New(users.Error.Message)
	}

	req, err = http.NewRequest("GET", "https://graph.microsoft.com/v1.0/groups?$select=id,displayName&$expand=members($select=id)&$top=1", nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var groups getGroupsResponse
	err = json.NewDecoder(resp.Body).Decode(&groups)
	if err != nil {
		return err
	}

	if groups.Error.Code != "" {
		return errors.New(groups.Error.Message)
	}

	return nil
}

func (a *Client) GetUsers(filters []string) ([]idp.User, error) {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	getUsersURL := "https://graph.microsoft.com/v1.0/users?$select=id,userPrincipalName,displayName,accountEnabled"
	if len(filters) > 0 {
		getUsersURL += "&" + buildPrefixFilter("userPrincipalName", filters)
	}

	var retval []idp.User
	for getUsersURL != "" {
		req, err := http.NewRequest("GET", getUsersURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+accessToken)
		req.Header.Add("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		var users getUsersResponse
		err = json.NewDecoder(resp.Body).Decode(&users)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		for _, user := range users.Value {
			retval = append(retval, idp.User{
				ID:              user.Id,
				Username:        user.UserPrincipalName,
				DisplayName:     user.DisplayName,
				AccountDisabled: !user.AccountEnabled,
			})
		}

		getUsersURL = users.NextLink
	}

	return retval, nil
}

func (a *Client) GetGroups(filters []string) ([]idp.Group, error) {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	getGroupsURL := "https://graph.microsoft.com/v1.0/groups?$select=id,displayName&$expand=members($select=id)"
	if len(filters) > 0 {
		getGroupsURL += "&" + buildPrefixFilter("displayName", filters)
	}

	var retval []idp.Group
	for getGroupsURL != "" {
		req, err := http.NewRequest("GET", getGroupsURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+accessToken)
		req.Header.Add("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		var groups getGroupsResponse
		err = json.NewDecoder(resp.Body).Decode(&groups)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		for _, group := range groups.Value {
			retvalMembers := make([]string, len(group.Members))
			for j, member := range group.Members {
				retvalMembers[j] = member.Id
			}

			retval = append(retval, idp.Group{
				ID:      group.Id,
				Name:    group.DisplayName,
				Members: retvalMembers,
			})
		}

		getGroupsURL = groups.NextLink
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
		return "", errors.New("invalid credentials")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var tokenResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	if token, ok := tokenResp["access_token"].(string); ok {
		return token, nil
	}

	return "", errors.New("invalid credentials")
}

func buildPrefixFilter(field string, prefixes []string) string {
	if len(prefixes) == 0 {
		return ""
	}

	if len(prefixes) == 1 {
		return fmt.Sprintf("$filter=startswith(%s,'%s')", field, prefixes[0])
	}

	return buildPrefixFilter(field, prefixes[:1]) + "%20or%20" + buildPrefixFilter(field, prefixes[1:])
}

type getUsersResponse struct {
	Error        errorResponse `json:"error"`
	OdataContext string        `json:"@odata.context"`
	Value        []struct {
		Id                string `json:"id"`
		UserPrincipalName string `json:"userPrincipalName"`
		DisplayName       string `json:"displayName"`
		AccountEnabled    bool   `json:"accountEnabled"`
	} `json:"value"`
	NextLink string `json:"@odata.nextLink"`
}

type getGroupsResponse struct {
	Error        errorResponse `json:"error"`
	OdataContext string        `json:"@odata.context"`
	Value        []struct {
		Id          string `json:"id"`
		DisplayName string `json:"displayName"`
		Members     []struct {
			OdataType string `json:"@odata.type"`
			Id        string `json:"id"`
		} `json:"members"`
	} `json:"value"`
	NextLink string `json:"@odata.nextLink"`
}

type errorResponse struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	InnerError struct {
		Date            string `json:"date"`
		RequestId       string `json:"request-id"`
		ClientRequestId string `json:"client-request-id"`
	} `json:"innerError"`
}
